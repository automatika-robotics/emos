package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/tlsca"
)

// withTempDirs points the config, recipes and UI security state at a fresh home.
func withTempDirs(t *testing.T) {
	t.Helper()
	saved := []*string{&config.HomeDir, &config.ConfigDir, &config.ConfigFile, &config.RecipesDir, &config.UISecurityDir}
	values := make([]string, len(saved))
	for i, p := range saved {
		values[i] = *p
	}
	home := t.TempDir()
	config.HomeDir = home
	config.ConfigDir = filepath.Join(home, ".config", "emos")
	config.ConfigFile = filepath.Join(config.ConfigDir, "config.json")
	config.RecipesDir = filepath.Join(home, "emos", "recipes")
	config.UISecurityDir = filepath.Join(home, "emos", ".ui-security")
	t.Cleanup(func() {
		for i, p := range saved {
			*p = values[i]
		}
	})
}

func envValue(env []string, key string) string {
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			return kv[len(key)+1:]
		}
	}
	return ""
}

func TestRunsGiveRecipeUIsTheRobotsCertificate(t *testing.T) {
	withTempDirs(t)
	s, err := newStrategy(&config.EMOSConfig{Mode: config.ModeNative}, "")
	if err != nil {
		t.Fatal(err)
	}
	env := s.Command("true").Env
	if got := envValue(env, uiDataDirEnv); got != config.UISecurityDir {
		t.Errorf("%s = %q, want %q", uiDataDirEnv, got, config.UISecurityDir)
	}
	robotCert, robotKey := tlsca.Paths()
	if cert := envValue(env, uiTLSCertEnv); cert != robotCert {
		t.Errorf("%s = %q, want the robot's %q", uiTLSCertEnv, cert, robotCert)
	}
	if key := envValue(env, uiTLSKeyEnv); key != robotKey {
		t.Errorf("%s = %q, want the robot's %q", uiTLSKeyEnv, key, robotKey)
	}
	// The run minted the certificate, as emos serve would.
	if _, err := tlsca.Load(); err != nil {
		t.Errorf("no certificate after a run: %v", err)
	}
	if info, _ := os.Stat(config.UISecurityDir); info.Mode().Perm() != 0o700 {
		t.Errorf("state directory mode = %o, want 700", info.Mode().Perm())
	}
	if info, _ := os.Stat(robotKey); info.Mode().Perm() != 0o600 {
		t.Errorf("key mode = %o, want 600", info.Mode().Perm())
	}
}

func TestContainerRunsSeeTheUISecurityStateUnderTheEmosMount(t *testing.T) {
	withTempDirs(t)
	s, err := newStrategy(&config.EMOSConfig{Mode: config.ModeOSSContainer}, "")
	if err != nil {
		t.Fatal(err)
	}
	shell := lastArg(s.Command("run"))
	for _, export := range []string{
		"export 'SUGARCOAT_UI_DATA_DIR=/emos/.ui-security'",
		"export 'SUGARCOAT_UI_TLS_CERT=/emos/.ui-security/tls.crt'",
		"export 'SUGARCOAT_UI_TLS_KEY=/emos/.ui-security/tls.key'",
	} {
		if !strings.Contains(shell, export) {
			t.Errorf("container command %q lacks %q", shell, export)
		}
	}
	// Those paths are the host's ~/emos/.ui-security through the mount.
	for _, name := range []string{"tls.crt", "tls.key"} {
		if _, err := os.Stat(filepath.Join(config.UISecurityDir, name)); err != nil {
			t.Errorf("%s is not on the host side of the mount: %v", name, err)
		}
	}
}
