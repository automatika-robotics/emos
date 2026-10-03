package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// ErrBusy is returned when another plugin install, update or removal holds the
// lock, from this process or another: the CLI and the dashboard share it.
var ErrBusy = errors.New("another plugin install, update or removal is in progress")

// Lock takes the plugin lock without waiting, failing with ErrBusy when it is
// held. Install, Update, Remove and RemoveAll must run under it; the returned
// function releases it.
func Lock() (func(), error) {
	if err := os.MkdirAll(config.ConfigDir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(config.ConfigDir, "plugins.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return func() { f.Close() }, nil
}

// Busy reports whether a plugin operation holds the lock. It takes the lock for
// an instant to find out, so a plugin operation starting at that moment may be
// told to retry.
func Busy() bool {
	unlock, err := Lock()
	if err != nil {
		return errors.Is(err, ErrBusy)
	}
	unlock()
	return false
}
