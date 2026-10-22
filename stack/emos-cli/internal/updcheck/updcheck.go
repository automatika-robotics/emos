// Package updcheck checks whether a newer EMOS release is available. It queries
// github release api and caches latest results with a TTL

package updcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// cacheFileName is the JSON document that holds the last successful fetch.
// Lives under config.ConfigDir.
const cacheFileName = "update-cache.json"

// DefaultRefreshInterval is the cadence at which both the CLI's
// RefreshIfStale TTL and the daemon's background ticker consult GitHub
// for the latest release tag. 6h sits well under the unauthenticated
// rate limit for Github.
const DefaultRefreshInterval = 6 * time.Hour

// httpClient bounds the GitHub lookup. The CLI runs interactively and the
// daemon refreshes asynchronously, so a slow response only stalls one
// goroutine.
var httpClient = &http.Client{Timeout: 5 * time.Second}

// refreshMu serialises in-process background refreshes started by
// RefreshIfStale.
var refreshMu sync.Mutex

// cacheFile struct on disk.
type cacheFile struct {
	Latest    string    `json:"latest"`
	Channel   string    `json:"channel,omitempty"` // the channel Latest was fetched on
	CheckedAt time.Time `json:"checked_at"`
}

// Latest fetches the newest version this binary's channel offers, without
// the leading "v".
func Latest(ctx context.Context) (string, error) {
	if config.Channel() == "dev" {
		return latestDev(ctx)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := fetchJSON(ctx, config.LatestReleaseURL(), &release); err != nil {
		return "", err
	}
	tag := strings.TrimPrefix(release.TagName, "v")
	if tag == "" {
		return "", errors.New("github releases API returned an empty tag_name")
	}
	return tag, nil
}

// latestDev picks the newest nightly, a pre-release tagged v<version>-dev.<date>,
// from the release list.
func latestDev(ctx context.Context) (string, error) {
	var releases []struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := fetchJSON(ctx, config.ReleaseListURL(), &releases); err != nil {
		return "", err
	}
	var latest string
	for _, r := range releases {
		version := strings.TrimPrefix(r.TagName, "v")
		if !r.Prerelease || !strings.Contains(version, "-dev.") {
			continue
		}
		if latest == "" || IsNewer(latest, version) {
			latest = version
		}
	}
	if latest == "" {
		return "", errors.New("no dev build has been published")
	}
	return latest, nil
}

// fetchJSON decodes the GitHub API document at url into v.
func fetchJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build releases request: %w", err)
	}
	// GitHub's API serves /releases/latest with HTML content negotiation
	// off by default, being explicit
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github releases API unreachable: %w", err)
	}
	defer resp.Body.Close()

	// Cap the body so a misconfigured upstream can't blow up memory
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read releases response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github releases API returned HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("parse releases response: %w", err)
	}
	return nil
}

// CachedLatest returns the last successful Latest() value and its
// timestamp. ok=false if the cache file is absent or unreadable.
func CachedLatest() (latest string, checkedAt time.Time, ok bool) {
	path := cachePath()
	if path == "" {
		return "", time.Time{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}, false
	}
	var c cacheFile
	if err := json.Unmarshal(data, &c); err != nil {
		return "", time.Time{}, false
	}
	// A cache the other channel's binary wrote names its newest, not ours
	if c.Latest == "" || c.Channel != config.Channel() {
		return "", time.Time{}, false
	}
	return c.Latest, c.CheckedAt, true
}

// RefreshIfStale returns the cached `latest` value and refreshes it when
// stale. Two refresh modes:
//   - On a cache miss (no prior value on disk) the function blocks for
//     up to firstFetchTimeout to perform a synchronous fetch.
//   - On a stale cache hit (value present but older than ttl) the
//     function returns the stale value immediately and refreshes
//     asynchronously.
func RefreshIfStale(ttl time.Duration) (cachedLatest string, cachedOK bool) {
	var checkedAt time.Time
	cachedLatest, checkedAt, cachedOK = CachedLatest()
	if cachedOK && time.Since(checkedAt) < ttl {
		return cachedLatest, cachedOK
	}
	// Only one refresh in flight at a time. On fail, assume another
	// goroutine is already doing the work. Return
	if !refreshMu.TryLock() {
		return cachedLatest, cachedOK
	}

	if !cachedOK {
		// Cache miss; block synchronously so the calling CLI command
		// has something to show on its first execution.
		defer refreshMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), firstFetchTimeout)
		defer cancel()
		fresh, err := Latest(ctx)
		if err != nil {
			return "", false
		}
		_ = writeCache(fresh, time.Now().UTC())
		return fresh, true
	}

	// Stale cache; return what we have, refresh in background.
	go func() {
		defer refreshMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), backgroundFetchTimeout)
		defer cancel()
		fresh, err := Latest(ctx)
		if err != nil {
			return
		}
		_ = writeCache(fresh, time.Now().UTC())
	}()
	return cachedLatest, cachedOK
}

// firstFetchTimeout bounds the blocking first-fetch.
var firstFetchTimeout = 2 * time.Second

// backgroundFetchTimeout bounds an async refresh of a stale cache. Higher
// than firstFetchTimeout because nobody is waiting on it.
var backgroundFetchTimeout = 8 * time.Second

// IsNewer reports whether latest is a newer version than current. Versions
// are X.Y.Z with an optional pre-release suffix, as in X.Y.Z-dev.*;
// a release is newer than its own pre-releases.
func IsNewer(current, latest string) bool {
	if current == "" || current == "dev" || latest == "" {
		return false
	}
	c, cpre, ok := parseVersion(current)
	if !ok {
		return false
	}
	l, lpre, ok := parseVersion(latest)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	switch {
	case lpre == cpre:
		return false
	case lpre == "":
		return true
	case cpre == "":
		return false
	}
	return newerPreRelease(cpre, lpre)
}

// parseVersion splits "X.Y.Z-dev.*" (or "vX.Y.Z") into [X, Y, Z] and
// the pre-release part. Returns ok=false if any of the first three
// dot-separated parts isn't a non-negative int.
func parseVersion(s string) ([3]int, string, bool) {
	s = strings.TrimPrefix(s, "v")
	var pre string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s, pre = s[:i], s[i+1:]
	}
	parts := strings.SplitN(s, ".", 4)
	if len(parts) < 3 {
		return [3]int{}, "", false
	}
	var out [3]int
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return [3]int{}, "", false
		}
		out[i] = n
	}
	return out, pre, true
}

// newerPreRelease compares dot-separated pre-release parts the semver way.
// Numbers numerically, anything else as strings, more parts win a tie.
func newerPreRelease(current, latest string) bool {
	c, l := strings.Split(current, "."), strings.Split(latest, ".")
	for i := 0; i < len(c) && i < len(l); i++ {
		if c[i] == l[i] {
			continue
		}
		cn, cerr := strconv.Atoi(c[i])
		ln, lerr := strconv.Atoi(l[i])
		if cerr == nil && lerr == nil {
			return ln > cn
		}
		return l[i] > c[i]
	}
	return len(l) > len(c)
}

// cachePath returns the absolute path of the cache file, or "" if
// config.ConfigDir is unset (which only happens before config.Init()).
func cachePath() string {
	if config.ConfigDir == "" {
		return ""
	}
	return filepath.Join(config.ConfigDir, cacheFileName)
}

// writeCache persists a successful fetch. Best-effort: errors are
// returned but callers ignore them (the next refresh will retry).
func writeCache(latest string, checkedAt time.Time) error {
	path := cachePath()
	if path == "" {
		return errors.New("config.ConfigDir not set")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(cacheFile{Latest: latest, Channel: config.Channel(), CheckedAt: checkedAt})
	if err != nil {
		return err
	}
	// Atomic write: Owned by user. Stale .tmp on a crashed write is rare
	// and harmless (next call truncates).
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
