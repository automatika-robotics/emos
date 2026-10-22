package installer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKompassCoreVerdict(t *testing.T) {
	cases := []struct {
		name, installed, latest string
		lookupErr               error
		current                 bool
		note                    string
	}{
		{"at the latest tag", "0.8.7", "0.8.7", nil, true, "is the latest"},
		{"a newer tag exists", "0.8.7", "0.8.10", nil, false, "0.8.10 is out"},
		{"a build past the latest tag is kept", "0.9.0", "0.8.7", nil, true, "is the latest"},
		{"missing or not loading", "", "0.8.7", nil, false, "does not load"},
		{"missing wins over a failed lookup", "", "", errors.New("offline"), false, "does not load"},
		{"the lookup failed", "0.8.7", "", errors.New("offline"), true, "could not be checked"},
	}
	for _, c := range cases {
		current, note := kompassCoreVerdict(c.installed, c.latest, c.lookupErr)
		if current != c.current || !strings.Contains(note, c.note) {
			t.Errorf("%s: got %v %q", c.name, current, note)
		}
	}
}

func TestKompassCoreCurrentReadsTheProbeAndTheTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"name": "0.8.7"}, {"name": "0.8.6"}]`))
	}))
	defer server.Close()
	old := kompassCoreTagsURL
	kompassCoreTagsURL = server.URL
	defer func() { kompassCoreTagsURL = old }()

	if tag, err := latestKompassCore(context.Background()); err != nil || tag != "0.8.7" {
		t.Fatalf("latest tag: %q %v", tag, err)
	}
	// The module may print while it loads; the version is the last line
	loads := func(string) (string, error) { return "[acpp] runtime up\n0.8.7\n", nil }
	if current, note := KompassCoreCurrent(loads); !current {
		t.Errorf("an installed latest should be kept: %q", note)
	}
	broken := func(string) (string, error) { return "", errors.New("exit status 1") }
	if current, note := KompassCoreCurrent(broken); current {
		t.Errorf("a module that does not load should be rebuilt: %q", note)
	}
}
