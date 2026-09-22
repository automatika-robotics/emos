package cmd

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// The dashboard answers plain HTTP on its HTTPS port with a redirect. The
// reload has to reach the daemon through that, not be turned into a GET by it.
func TestNotifyDaemonReloadAuthReachesAnHTTPSDaemon(t *testing.T) {
	origDir, origCfg, origLic := config.ConfigDir, config.ConfigFile, config.LicenseFile
	t.Cleanup(func() { config.ConfigDir, config.ConfigFile, config.LicenseFile = origDir, origCfg, origLic })
	config.ConfigDir = filepath.Join(t.TempDir(), ".config", "emos")
	config.ConfigFile = filepath.Join(config.ConfigDir, "config.json")
	config.LicenseFile = filepath.Join(config.ConfigDir, "license.json")

	var reloads, plainHits atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			plainHits.Add(1)
			http.Redirect(w, r, "https://"+r.Host+r.URL.Path, http.StatusTemporaryRedirect)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/reload-auth" {
			reloads.Add(1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	// One port, TLS and plain on it, as the dashboard does it: the TLS server
	// on the port, and the plain redirect answered by the same handler.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	daemon := httptest.NewUnstartedServer(handler)
	daemon.Listener = ln
	daemon.TLS = &tls.Config{}
	daemon.StartTLS()
	t.Cleanup(daemon.Close)
	port, _ := strconv.Atoi(daemon.URL[len("https://127.0.0.1:"):])
	if err := config.SaveConfig(&config.EMOSConfig{Mode: config.ModePixi, Port: port}); err != nil {
		t.Fatal(err)
	}

	if !notifyDaemonReloadAuth() {
		t.Fatal("the reload must be reported as done")
	}
	if reloads.Load() != 1 {
		t.Errorf("the daemon reloaded %d times, want 1", reloads.Load())
	}
	if plainHits.Load() != 0 {
		t.Errorf("the reload went over plain HTTP %d times and was redirected", plainHits.Load())
	}
}

func TestNotifyDaemonReloadAuthWithoutADaemon(t *testing.T) {
	origDir, origCfg, origLic := config.ConfigDir, config.ConfigFile, config.LicenseFile
	t.Cleanup(func() { config.ConfigDir, config.ConfigFile, config.LicenseFile = origDir, origCfg, origLic })
	config.ConfigDir = filepath.Join(t.TempDir(), ".config", "emos")
	config.ConfigFile = filepath.Join(config.ConfigDir, "config.json")
	config.LicenseFile = filepath.Join(config.ConfigDir, "license.json")
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // nothing listens there now
	if err := config.SaveConfig(&config.EMOSConfig{Mode: config.ModePixi, Port: port}); err != nil {
		t.Fatal(err)
	}
	if notifyDaemonReloadAuth() {
		t.Error("no daemon must be reported as not reloaded")
	}
}
