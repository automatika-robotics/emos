package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/automatika-robotics/emos-cli/internal/config"
	"github.com/automatika-robotics/emos-cli/internal/container"
)

// RunUISecurity runs Sugarcoat's ui_security tool with args in the environment
// recipes run in, so it acts on the keys their UIs use. Output goes to out.
func RunUISecurity(cfg *config.EMOSConfig, out io.Writer, args ...string) error {
	s, err := newStrategy(cfg, "")
	if err != nil {
		return err
	}
	// Runs in the recipes' container, which a run would have started.
	if c, ok := s.(*ContainerStrategy); ok {
		if err := c.ensureRunning(); err != nil {
			return err
		}
	}
	cmd := s.Command(uiSecurityShell(args))
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		// The tool has said what went wrong.
		return fmt.Errorf("ui_security exited with status %d", exit.ExitCode())
	}
	return err
}

func uiSecurityShell(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(append([]string{"ros2 run automatika_ros_sugar ui_security"}, quoted...), " ")
}

// ensureRunning starts the container if it is stopped, without the restart a
// run does.
func (s *ContainerStrategy) ensureRunning() error {
	if !container.Exists(config.ContainerName) {
		return fmt.Errorf("container '%s' does not exist — run 'emos install' first", config.ContainerName)
	}
	if container.IsRunning(config.ContainerName) {
		return nil
	}
	return container.Start(config.ContainerName)
}
