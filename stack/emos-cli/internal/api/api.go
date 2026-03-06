package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()

	if err := unzip(zipPath, destDir); err != nil {
		os.Remove(zipPath)
		return fmt.Errorf("failed to extract recipe: %w", err)
	}

	os.Remove(zipPath)
	return nil
}

func unzip(src, dest string) error {
	return exec.Command("unzip", "-o", src, "-d", dest).Run()
}
