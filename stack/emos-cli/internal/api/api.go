package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

type Credentials struct {
	Registry       string `json:"container_registry"`
	ImageName      string `json:"image_name"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	DeploymentRepo string `json:"deployment_repository_name"`
}

func (c *Credentials) FullImage() string {
	return c.Registry + "/" + c.ImageName
}

type Recipe struct {
	Filename string `json:"filename"`
	Name     string `json:"name"`
}

type apiError struct {
	Error string `json:"error"`
}

func ValidateLicense(key string) (*Credentials, error) {
	body := fmt.Sprintf(`{"license_key": "%s"}`, key)
	resp, err := http.Post(config.CredentialsEndpoint, "application/json", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not connect to the license API: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read API response: %w", err)
	}

	var apiErr apiError
	if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
		return nil, fmt.Errorf("API error: %s", apiErr.Error)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("invalid API response: %w", err)
	}

	if creds.Registry == "" || creds.Username == "" || creds.Password == "" || creds.ImageName == "" || creds.DeploymentRepo == "" {
		return nil, fmt.Errorf("API returned incomplete credentials")
	}

	return &creds, nil
}

func ListRecipes() ([]Recipe, error) {
	resp, err := http.Get(config.RecipesEndpoint)
	if err != nil {
		return nil, fmt.Errorf("could not connect to the recipes API: %w", err)
	}
	defer resp.Body.Close()

	var recipes []Recipe
	if err := json.NewDecoder(resp.Body).Decode(&recipes); err != nil {
		return nil, fmt.Errorf("invalid recipes API response: %w", err)
	}
	return recipes, nil
}

// WARN: DownloadRecipe fetches the recipe archive from the catalog and extracts it
// into <destDir>/<name>/. The upstream archive layout is inconsistent:
// Normalise both shapes to <destDir>/<name>/{manifest.json, recipe.py, ...}
// because that is the layout `emos run` expects.
// TODO: Make upstream layout consistent
func DownloadRecipe(name, destDir string) error {
	url := config.RecipesEndpoint + "/" + name
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("could not download recipe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	zipPath := filepath.Join(os.TempDir(), name+".zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	written, err := io.Copy(f, resp.Body)
	if err != nil {
		f.Close()
		os.Remove(zipPath)
		return fmt.Errorf("could not save recipe archive: %w", err)
	}
	f.Close()
	if written < 4 {
		os.Remove(zipPath)
		return fmt.Errorf("recipe archive is empty (%d bytes)", written)
	}

	target := filepath.Join(destDir, name)
	_ = os.RemoveAll(target)
	if err := unzipRecipeArchive(zipPath, target); err != nil {
		os.Remove(zipPath)
		return fmt.Errorf("failed to extract recipe: %w", err)
	}

	os.Remove(zipPath)
	return nil
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
		entryName := f.Name
		if stripPrefix != "" {
			entryName = strings.TrimPrefix(entryName, stripPrefix)
			if entryName == "" {
				continue // the wrapper directory itself
			}
		}

		target := filepath.Join(cleanDest, entryName)
		if !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) && target != cleanDest {
			return fmt.Errorf("zip entry %q escapes destination", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
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
