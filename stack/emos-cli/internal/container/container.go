package container

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func docker(args ...string) *exec.Cmd {
	return exec.Command("docker", args...)
}

func run(args ...string) (string, error) {
	cmd := docker(args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func Exists(name string) bool {
	_, err := run("inspect", name)
	return err == nil
}

func IsRunning(name string) bool {
	out, err := run("inspect", "-f", "{{.State.Status}}", name)
	return err == nil && out == "running"
}

func Status(name string) string {
	out, err := run("inspect", "-f", "{{.State.Status}}", name)
	if err != nil {
		return "not found"
	}
	return out
}

func Start(name string) error {
	_, err := run("start", name)
	return err
}

func Stop(name string) error {
	_, err := run("stop", "-t", "2", name)
	return err
}

func Remove(name string) error {
	_, err := run("rm", "-f", name)
	return err
}

func Login(registry, user, token string) error {
	cmd := docker("login", registry, "-u", user, "--password-stdin")
	cmd.Stdin = strings.NewReader(token)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker login failed: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func Pull(image string) error {
	cmd := docker("pull", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Run(name, image string, extraArgs ...string) error {
	args := []string{
		"run", "-d", "-it",
		"--restart", "always",
		"--privileged",
		"-v", "/dev/bus/usb:/dev/bus/usb",
		"-v", os.Getenv("HOME") + "/emos:/emos",
		"--name", name,
		"--network", "host",
		"--runtime", "nvidia",
		"--gpus=all",
	}
	args = append(args, extraArgs...)
	args = append(args, image)
	_, err := run(args...)
	return err
}

func RunWithArgs(name, image string, args []string) error {
	fullArgs := append([]string{"run"}, args...)
	fullArgs = append(fullArgs, "--name", name, image)
	_, err := run(fullArgs...)
	return err
}

func Exec(name, command string) (string, error) {
	return run("exec", name, "bash", "-c", command)
}

func ExecDetached(name, command string) error {
	_, err := run("exec", "-d", name, "bash", "-c", command)
	return err
}

func ExecInteractive(name, command string) error {
	cmd := docker("exec", "-it", name, "bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Cp(name, src, dst string) error {
	_, err := run("cp", src, name+":"+dst)
	return err
}

func CpFrom(name, src, dst string) error {
	_, err := run("cp", name+":"+src, dst)
	return err
}

func Top(name string) (string, error) {
	return run("top", name)
}

func Restart(name string) error {
	_, err := run("restart", name)
	return err
}

func FileExists(name, path string) bool {
	_, err := Exec(name, fmt.Sprintf("test -f '%s'", path))
	return err == nil
}

func DirExists(name, path string) bool {
	_, err := Exec(name, fmt.Sprintf("test -d '%s'", path))
	return err == nil
}
