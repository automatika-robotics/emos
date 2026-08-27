package plugin

import "testing"

func TestParseRole(t *testing.T) {
	cases := map[string]string{
		`{"role":"sensor"}`:            "sensor",
		`{"role":"robot"}`:             "robot",
		`{"role":"PluginRole.SENSOR"}`: "sensor", // tolerate an enum-style value
		`{"role":" Robot "}`:           "robot",  // trimmed + lowercased
		`{"metadata":{}}`:              "",
		`not json`:                     "",
	}
	for in, want := range cases {
		if got := parseRole([]byte(in)); got != want {
			t.Errorf("parseRole(%q) = %q, want %q", in, got, want)
		}
	}
}
