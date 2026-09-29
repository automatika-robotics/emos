package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/updcheck"
)

// kompassCoreTagsURL lists kompass-core's tags, newest first.
var kompassCoreTagsURL = "https://api.github.com/repos/automatika-robotics/kompass-core/tags?per_page=1"

// kompassCoreProbe prints the installed version, and only when the compiled
// module still loads in the environment.
const kompassCoreProbe = `python3 -c 'import importlib.metadata as m, kompass_cpp; print(m.version("kompass-core"))'`

// LatestKompassCore is the newest kompass-core tag.
func LatestKompassCore(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kompassCoreTagsURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", err
	}
	if len(tags) == 0 || tags[0].Name == "" {
		return "", fmt.Errorf("no tags")
	}
	return tags[0].Name, nil
}

// KompassCoreCurrent reports whether an update can leave kompass-core alone.
func KompassCoreCurrent(probe func(script string) (string, error)) (bool, string) {
	out, err := probe(kompassCoreProbe)
	if err != nil {
		out = ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	latest, err := LatestKompassCore(ctx)
	return kompassCoreVerdict(lastLine(out), latest, err)
}

func kompassCoreVerdict(installed, latest string, lookupErr error) (bool, string) {
	switch {
	case installed == "":
		return false, "kompass-core is missing or does not load, building it"
	case lookupErr != nil:
		return true, "kompass-core " + installed + " kept, its latest release could not be checked"
	case updcheck.IsNewer(installed, latest):
		return false, "kompass-core " + strings.TrimPrefix(latest, "v") + " is out, replacing " + installed
	}
	return true, "kompass-core " + installed + " is the latest, not rebuilding"
}

func lastLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
