package server

import (
	"errors"
	"log/slog"
	"net"
	"strings"

	"github.com/grandcat/zeroconf"
)

const (
	mdnsServiceType  = "_emos._tcp"
	mdnsHostname     = "emos" // resolvable as "emos.local" on the LAN
	mdnsInstanceName = "EMOS"
)

// announceMDNS publishes the dashboard on mDNS so any client on the LAN
// can reach the device as `emos.local:8765` (modulo client mDNS support).
//
// We use RegisterProxy rather than Register because Register publishes the
// HOST machine's hostname
// RegisterProxy publishes:
//
//   - PTR _emos._tcp.local. -> EMOS._emos._tcp.local.
//   - SRV ...               -> emos.local. port
//   - A    emos.local.      -> <each local IPv4>
//
// so any mDNS-aware client resolves `emos.local` directly.
//
// Soft-fails on hosts without a usable interface (containers, isolated
// namespaces).
func announceMDNS(port int, txt []string, log *slog.Logger) (*zeroconf.Server, error) {
	ips := LocalIPv4Strings()
	if len(ips) == 0 {
		log.Warn("mDNS unavailable: no usable IPv4 interfaces")
		return nil, nil
	}

	server, err := zeroconf.RegisterProxy(
		mdnsInstanceName,
		mdnsServiceType,
		"local.",
		port,
		mdnsHostname,
		ips,
		txt,
		nil, // all interfaces
	)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			log.Warn("mDNS unavailable", "err", err)
			return nil, nil
		}
		return nil, err
	}

	log.Info("mDNS announced",
		"hostname", mdnsHostname+".local",
		"service", mdnsServiceType,
		"port", port,
		"ips", strings.Join(ips, ","))
	return server, nil
}

// LocalIPv4Strings returns the LAN IPv4 addresses worth advertising on
// mDNS / printing in the boot banner. Exclude loopback and virtual interfaces.
// Keep `tailscale0` and physical adapters (eth/wlan/enp/wlp)
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
		if isVirtualIface(iface.Name) {
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
			ip4 := ipn.IP.To4()
			if ip4 == nil {
				continue
			}
			out = append(out, ip4.String())
		}
	}
	return out
}

func isVirtualIface(name string) bool {
	for _, prefix := range []string{"docker", "br-", "veth", "virbr", "cni", "podman", "kube"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// PreferredLANIPv4 returns the single best IPv4 to encode in a QR code, a
// physical Ethernet/Wi-Fi address that any phone on the same LAN can hit.
// Returns "" if no usable interface exists
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
		if isVirtualIface(iface.Name) {
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
			} else {
				if lan == "" {
					lan = ip4.String()
				}
			}
		}
	}
	if lan != "" {
		return lan
	}
	return vpn
}
