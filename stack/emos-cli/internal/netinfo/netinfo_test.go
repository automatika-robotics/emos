package netinfo

import "testing"

func TestIsVirtualIface(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		// Virtual / container networking — must be filtered.
		{"docker0", true},
		{"br-1234567890ab", true},
		{"veth1234", true},
		{"virbr0", true},
		{"cni0", true},
		{"podman0", true},
		{"kube-bridge", true},

		// Physical / VPN interfaces — must NOT be filtered. These are surfaced
		// to mDNS and the QR code; filtering them by accident would silently
		// hide the dashboard from phones on the LAN.
		{"eth0", false},
		{"eno1", false},
		{"enp3s0", false},
		{"wlan0", false},
		{"wlp2s0", false},
		{"tailscale0", false},
		{"wg0", false},
		{"tun0", false},
		{"lo", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsVirtualIface(tc.name)
			if got != tc.want {
				t.Fatalf("IsVirtualIface(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestEnumerationDoesNotPanic is a smoke test: the helpers query the host's
// real interfaces via net.Interfaces, which we can't meaningfully mock in
// CI. The minimum we want from CI is that none of them panic regardless of
// the runner's network setup (containers, no IPv4, no MAC, etc.).
func TestEnumerationDoesNotPanic(t *testing.T) {
	_ = LocalIPv4Strings()
	_ = PreferredLANIPv4()
	_ = PrimaryMAC()
}
