package plugin

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// txn changes the workspace so that a failure can put it back.
//
// New clones are fetched into a staging dir first. Old verlay is moved aside
// rather than deleted, and only discarded once the new overlay has built and
// been inspected. The staging dir sits beside the workspace, on the same
// filesystem so moves are renames, and outside it so colcon never discovers
// what is staged or set aside.
type txn struct {
	dir  string
	held int
	undo []func()
	done bool
}

func txnDir() string {
	return filepath.Join(filepath.Dir(config.WorkspaceDir), ".plugin-txn")
}

// begin starts a transaction. Leftovers from an operation that was killed
// before it could finish are dropped: there is no telling what state they
// describe.
func begin() (*txn, error) {
	t := &txn{dir: txnDir()}
	if err := os.RemoveAll(t.dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(t.dir, "new"), 0o755); err != nil {
		return nil, err
	}
	return t, nil
}

// staged is where a clone named name is fetched before it is placed.
func (t *txn) staged(name string) string {
	return filepath.Join(t.dir, "new", name)
}

// setAside moves path out of the way until commit. Whatever is created at path
// afterwards is removed again on rollback, including when path did not exist.
func (t *txn) setAside(path string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		t.undo = append(t.undo, func() { os.RemoveAll(path) })
		return nil
	}
	held := filepath.Join(t.dir, strconv.Itoa(t.held))
	t.held++
	if err := os.Rename(path, held); err != nil {
		return err
	}
	t.undo = append(t.undo, func() {
		os.RemoveAll(path)
		os.Rename(held, path)
	})
	return nil
}

// place moves the staged clone name into the workspace's source dir.
func (t *txn) place(name string) error {
	dst := filepath.Join(config.PluginSrcDir(), name)
	if err := t.setAside(dst); err != nil {
		return err
	}
	return os.Rename(t.staged(name), dst)
}

// onRollback registers an undo step for a change made in place.
func (t *txn) onRollback(undo func()) {
	t.undo = append(t.undo, undo)
}

// rollback undoes every change in reverse order. It is a no-op after commit, so
// it can be deferred.
func (t *txn) rollback() {
	if t.done {
		return
	}
	t.done = true
	for i := len(t.undo) - 1; i >= 0; i-- {
		t.undo[i]()
	}
	os.RemoveAll(t.dir)
}

// commit keeps the changes and discards what was set aside.
func (t *txn) commit() {
	t.done = true
	os.RemoveAll(t.dir)
}
