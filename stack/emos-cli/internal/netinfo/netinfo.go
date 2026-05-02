// Package netinfo collects small helpers that inspect the host's network
// interfaces. Used by mDNS publishing, the dashboard boot banner, and the
// identity module that derives a stable per-device name from a MAC address.
package netinfo

import (
	"net"
	"strings"
)

// IsVirtualIface reports whether an interface name belongs to one of the
// container/VM/CNI-style virtual networks we never want to advertise. Cheap
// prefix match; the list grows as new container runtimes appear in the wild.
func IsVirtualIface(name string) bool {
	for _, prefix := range []string{"docker", "br-", "veth", "virbr", "cni", "podman", "kube"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// LocalIPv4Strings returns the LAN IPv4 addresses worth advertising on mDNS
// or printing in the boot banner. Loopback and virtual interfaces are
// excluded; tailscale / wireguard / wlan / eth all qualify.
func LocalIPv4Strings() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if IsVirtualIface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ip4 := ipn.IP.To4(); ip4 != nil {
				out = append(out, ip4.String())
			}
		}
	}
	return out
}

// PreferredLANIPv4 returns the single best IPv4 to encode in a QR code: a
// physical Ethernet/Wi-Fi address that any phone on the same LAN can hit.
// Tailscale / wireguard / tun fall back behind LAN because a phone scanning
// the QR is presumably on local Wi-Fi, not the tailnet. Returns "" if no
// usable interface exists.
func PreferredLANIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var lan, vpn string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if IsVirtualIface(iface.Name) {
			continue
		}
		isVPN := strings.HasPrefix(iface.Name, "tailscale") ||
			strings.HasPrefix(iface.Name, "wg") ||
			strings.HasPrefix(iface.Name, "tun")
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipn.IP.To4()
			if ip4 == nil {
				continue
			}
			if isVPN {
				if vpn == "" {
					vpn = ip4.String()
				}
			} else if lan == "" {
				lan = ip4.String()
			}
		}
	}
	if lan != "" {
		return lan
	}
	return vpn
}

// PrimaryMAC returns a stable hardware address suitable for fingerprinting
// the device. Picks the first non-loopback, non-virtual, non-VPN interface
// with a hardware address. Returns "" if none qualify (e.g. running inside
// a stripped container).
func PrimaryMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if IsVirtualIface(iface.Name) {
			continue
		}
		if strings.HasPrefix(iface.Name, "tailscale") ||
			strings.HasPrefix(iface.Name, "wg") ||
			strings.HasPrefix(iface.Name, "tun") {
			continue
		}
		if hw := iface.HardwareAddr; len(hw) > 0 {
			return hw.String()
		}
	}
	return ""
}
