package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

type Recipe struct {
	Filename    string          `json:"filename"` // the name a recipe is pulled and run by
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	Variants    []RecipeVariant `json:"variants"`
}

// RecipeVariant is one version of a recipe: the generic one, which runs on any
// robot, or one written for a robot plugin and the sensor plugins it names.
type RecipeVariant struct {
	ID      string   `json:"id"`      // opaque, only for the download URL
	Robot   string   `json:"robot"`   // robot plugin slug, empty for the generic variant
	Sensors []string `json:"sensors"` // sensor plugin slugs the variant needs
}

// GenericVariant is the id of the variant that needs no license.
const GenericVariant = "generic"

// ErrNoSuchRecipe is the portal's answer for a recipe or a variant it does not have.
var ErrNoSuchRecipe = errors.New("the portal has no such recipe")

// RecipeRefusedError is why the portal will not give a robot variant to the
// license it was asked with, in the portal's words.
type RecipeRefusedError struct{ Reason string }

func (e *RecipeRefusedError) Error() string { return e.Reason }

// ErrInvalidLicense is the portal's answer to a key it does not know or has
// deactivated. It does not say which of the two.
var ErrInvalidLicense = errors.New("invalid or inactive license key")

// ErrLicenseNotClaimed is the answer for a real key that its holder has not
// activated on the support portal, which is where the licence agreement is
// accepted. Such a key is not verified.
var ErrLicenseNotClaimed = errors.New("this license has not been activated yet: sign in at " + config.SupportURL +
	", activate it with the robot's serial number and this key, accept the license agreement, then run this again")

// licenseClient bounds the one request that runs before an install starts.
var licenseClient = &http.Client{Timeout: 20 * time.Second}

// VerifyLicense asks the portal what key entitles its holder to.
func VerifyLicense(key string) (*config.License, error) {
	key = strings.TrimSpace(key)
	body, err := json.Marshal(map[string]string{"license_key": key})
	if err != nil {
		return nil, err
	}
	resp, err := licenseClient.Post(config.VerifyEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not reach the license server: %w", err)
	}
	defer resp.Body.Close()
	return parseVerifyResponse(resp, key, time.Now().UTC().Truncate(time.Second))
}

func parseVerifyResponse(resp *http.Response, key string, now time.Time) (*config.License, error) {
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, ErrInvalidLicense
	case http.StatusTooManyRequests:
		return nil, errors.New("too many license checks from this network, try again in a minute")
	default:
		return nil, fmt.Errorf("the license server answered %s", resp.Status)
	}

	var answer struct {
		Valid     bool `json:"valid"`
		IsClaimed bool `json:"is_claimed"`
		config.License
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return nil, fmt.Errorf("the license server sent an answer that could not be read: %w", err)
	}
	if !answer.Valid {
		return nil, ErrInvalidLicense
	}
	if !answer.IsClaimed {
		return nil, ErrLicenseNotClaimed
	}
	if answer.PluginSlug == "" {
		return nil, errors.New("the license server did not name the robot this license is for")
	}
	lic := answer.License
	lic.Key = key
	lic.VerifiedAt = now
	return &lic, nil
}

func ListRecipes() ([]Recipe, error) {
	resp, err := http.Get(config.RecipesEndpoint)
	if err != nil {
		return nil, fmt.Errorf("could not connect to the recipes API: %w", err)
	}
	defer resp.Body.Close()
	return parseRecipesResponse(resp)
}

// parseRecipesResponse validates and decodes a /recipes catalog response.
func parseRecipesResponse(resp *http.Response) ([]Recipe, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read recipes API response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"recipe catalog unavailable (HTTP %d). The service may be down or upgrading -- try again later",
			resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "json") {
		return nil, fmt.Errorf(
			"recipe catalog returned a non-JSON response. The service may be down or upgrading -- try again later")
	}

	var recipes []Recipe
	if err := json.Unmarshal(body, &recipes); err != nil {
		return nil, fmt.Errorf(
			"recipe catalog returned an unexpected response. The service may be down or upgrading -- try again later")
	}
	return recipes, nil
}

// Plugin is a robot-plugin registry entry from the support portal. Sources in
// github repos.
type Plugin struct {
	Filename    string   `json:"filename"` // slug / clone-directory name
	Name        string   `json:"name"`
	Vendor      string   `json:"vendor"`
	Role        string   `json:"role"`        // "robot" | "sensor" — catalog badge + install routing
	EntryPoint  string   `json:"entry_point"` // module:ClassName
	Repo        string   `json:"repo"`        // public GitHub URL
	Ref         string   `json:"ref"`         // branch/tag; empty = default branch
	Image       string   `json:"image"`       // image filename served by the portal, if any
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// ListPlugins fetches the robot-plugin registry from the support portal.
func ListPlugins() ([]Plugin, error) {
	resp, err := http.Get(config.PluginsEndpoint)
	if err != nil {
		return nil, fmt.Errorf("could not connect to the plugins API: %w", err)
	}
	defer resp.Body.Close()
	return parsePluginsResponse(resp)
}

// parsePluginsResponse validates and decodes a /plugins catalog response.
func parsePluginsResponse(resp *http.Response) ([]Plugin, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins API response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"plugin catalog unavailable (HTTP %d). The service may be down or upgrading -- try again later",
			resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "json") {
		return nil, fmt.Errorf(
			"plugin catalog returned a non-JSON response. The service may be down or upgrading -- try again later")
	}

	var plugins []Plugin
	if err := json.Unmarshal(body, &plugins); err != nil {
		return nil, fmt.Errorf(
			"plugin catalog returned an unexpected response. The service may be down or upgrading -- try again later")
	}
	return plugins, nil
}

// DownloadRecipe fetches one variant of a recipe and unpacks it to
// <recipesDir>/<name>, replacing what is there. A robot variant needs the
// license key; the generic one takes none.
//
// WARN: The upstream archive layout is inconsistent. Both shapes are normalised
// to <recipesDir>/<name>/{manifest.json, recipe.py, ...}, the layout `emos run` expects.
// TODO: Make upstream layout consistent
func DownloadRecipe(ctx context.Context, name, variant, licenseKey, recipesDir string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, recipeURL(name, variant), nil)
	if err != nil {
		return fmt.Errorf("build recipe request: %w", err)
	}
	if licenseKey != "" {
		req.Header.Set("X-EMOS-License", licenseKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not download recipe: %w", err)
	}
	defer resp.Body.Close()
	if err := checkRecipeResponse(resp); err != nil {
		return err
	}

	// Use a per-call randomised tempfile so concurrent pulls of the same
	// recipe don't race over the same path.
	f, err := os.CreateTemp("", "emos-recipe-"+name+"-*.zip")
	if err != nil {
		return err
	}
	zipPath := f.Name()
	defer os.Remove(zipPath)
	written, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		return fmt.Errorf("could not save recipe archive: %w", err)
	}
	if written < 4 {
		return fmt.Errorf("recipe archive is empty (%d bytes)", written)
	}

	target := filepath.Join(recipesDir, name)
	_ = os.RemoveAll(target)
	if err := unzipRecipeArchive(zipPath, target); err != nil {
		return fmt.Errorf("failed to extract recipe: %w", err)
	}
	return nil
}

func recipeURL(name, variant string) string {
	return config.RecipesEndpoint + "/" + url.PathEscape(name) + "/" + url.PathEscape(variant)
}

// checkRecipeResponse turns the portal's refusals into errors a caller can tell apart.
func checkRecipeResponse(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return ErrInvalidLicense
	case http.StatusNotFound:
		return ErrNoSuchRecipe
	case http.StatusForbidden:
		var body struct {
			Detail string `json:"detail"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body) == nil && body.Detail != "" {
			return &RecipeRefusedError{Reason: body.Detail}
		}
		return &RecipeRefusedError{Reason: "the portal refused this license for the recipe"}
	case http.StatusTooManyRequests:
		return errors.New("too many recipe downloads from this network, try again in a minute")
	}
	return fmt.Errorf("download failed with status %d", resp.StatusCode)
}

// unzipRecipeArchive extracts a recipe zip into destDir//
// Refuses any entry whose resolved path escapes destDir (zip-slip guard).
func unzipRecipeArchive(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	cleanDest, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	stripPrefix := commonTopLevelDir(zr.File)

	for _, f := range zr.File {
		if err := extractZipEntry(f, cleanDest, stripPrefix); err != nil {
			return err
		}
	}
	return nil
}

// extractZipEntry handles a single archive entry. Pulling this out keeps the
// per-entry resource lifetimes tidy.
func extractZipEntry(f *zip.File, destDir, stripPrefix string) error {
	entryName := f.Name
	if stripPrefix != "" {
		entryName = strings.TrimPrefix(entryName, stripPrefix)
		if entryName == "" {
			return nil // the wrapper directory itself
		}
	}

	target := filepath.Join(destDir, entryName)
	if !strings.HasPrefix(target, destDir+string(os.PathSeparator)) && target != destDir {
		return fmt.Errorf("zip entry %q escapes destination", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, f.Mode())
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// commonTopLevelDir returns the shared first-segment-and-slash prefix of every
// non-empty zip entry, or "" if entries are flat (some at root)
func commonTopLevelDir(files []*zip.File) string {
	var prefix string
	for _, f := range files {
		if f.Name == "" {
			continue
		}
		idx := strings.IndexByte(f.Name, '/')
		if idx < 0 {
			return "" // an entry at the root -> archive is already flat
		}
		first := f.Name[:idx+1]
		if prefix == "" {
			prefix = first
		} else if prefix != first {
			return "" // multiple top-level dirs -> don't try to be clever
		}
	}
	return prefix
}
