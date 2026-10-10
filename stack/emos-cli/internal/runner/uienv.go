package runner

import (
	"fmt"
	"path/filepath"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/tlsca"
)

// Sugarcoat reads these when a recipe calls enable_ui.
const (
	uiDataDirEnv = "SUGARCOAT_UI_DATA_DIR"
	uiTLSCertEnv = "SUGARCOAT_UI_TLS_CERT"
	uiTLSKeyEnv  = "SUGARCOAT_UI_TLS_KEY"
)

// uiEnv returns the environment giving recipe UIs their state directory, which
// holds their API keys and the robot's certificate, so they present the same
// certificate as the dashboard.
//
// The directory is under ~/emos, which recipes started from the dashboard
// service can write to, and which a container keeps across updates.
func uiEnv(dir string) ([]string, error) {
	deviceName, err := config.ResolveDeviceName()
	if err != nil {
		return nil, fmt.Errorf("device name: %w", err)
	}
	if _, err := tlsca.Ensure(deviceName); err != nil {
		return nil, fmt.Errorf("robot TLS certificate: %w", err)
	}
	cert, key := tlsca.Paths()
	return []string{
		uiDataDirEnv + "=" + dir,
		uiTLSCertEnv + "=" + filepath.Join(dir, filepath.Base(cert)),
		uiTLSKeyEnv + "=" + filepath.Join(dir, filepath.Base(key)),
	}, nil
}
