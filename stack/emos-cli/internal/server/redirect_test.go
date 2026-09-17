package server

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

// serveOnFreePort runs a dashboard on a free port until the test ends.
func serveOnFreePort(t *testing.T, disableTLS bool) string {
	t.Helper()
	withTempConfig(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	s, err := New(Options{
		Addr: addr, DeviceName: "dev", DisableMDNS: true, DisableTLS: disableTLS,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Error("Run did not return after cancel")
		}
	})

	deadline := time.Now().Add(10 * time.Second)
	for {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			c.Close()
			return addr
		}
		if time.Now().After(deadline) {
			t.Fatalf("the dashboard never listened on %s", addr)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestPlainHTTPOnTheDashboardPortIsRedirectedToHTTPS(t *testing.T) {
	addr := serveOnFreePort(t, false)

	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := noFollow.Get("http://" + addr + "/recipes?tab=installed")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("plain HTTP got %d, want a temporary redirect", resp.StatusCode)
	}
	if got, want := resp.Header.Get("Location"), "https://"+addr+"/recipes?tab=installed"; got != want {
		t.Errorf("redirected to %q, want %q", got, want)
	}

	secure := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	resp, err = secure.Get("https://" + addr + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HTTPS health got %d, want 200", resp.StatusCode)
	}
}

func TestNoTLSServesPlainHTTP(t *testing.T) {
	addr := serveOnFreePort(t, true)
	resp, err := http.Get("http://" + addr + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("plain HTTP health got %d, want 200", resp.StatusCode)
	}
}
