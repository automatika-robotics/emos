package server

import (
	"errors"
	"log/slog"
	"net"
	"strings"

	"github.com/grandcat/zeroconf"

	"github.com/automatika-robotics/emos-cli/internal/netinfo"
)

const (
	mdnsServiceType   = "_emos._tcp"
	mdnsSharedHost    = "emos" // shared shortcut: any EMOS device on the LAN
	mdnsSharedInstanceLabel = "EMOS"
)

// mdnsRegistrations is the set of zeroconf servers we keep alive for the
// duration of `emos serve`. Bundled together so Server.Run can shut them
// down cleanly on SIGTERM.
type mdnsRegistrations struct {
	servers []*zeroconf.Server
}

// Shutdown closes every still-open registration. Safe to call on a nil receiver.
func (m *mdnsRegistrations) Shutdown() {
	if m == nil {
		return
	}
	for _, s := range m.servers {
		if s != nil {
			s.Shutdown()
		}
	}
}

// announceMDNS publishes the dashboard on mDNS under TWO names:
//
//  1. <deviceName>.local  — unique per device, derived from a stable
//     hardware fingerprint, used to disambiguate robots in fleets.
//  2. emos.local           — shared shortcut so a customer with a single
//     robot can type a memorable address; on multi-robot LANs the
//     resolver picks one (last-writer-wins behaviour by Avahi/Bonjour
//     is acceptable for "any of my robots").
//
// Service-discovery tools (`dns-sd -B _emos._tcp`) see both registrations
// listed as separate instances: `EMOS-<deviceName>` and `EMOS`.
//
// Soft-fails on hosts without a usable interface (containers, isolated
// namespaces); the foreground URLs still work via direct IP.
func announceMDNS(port int, deviceName string, txt []string, log *slog.Logger) (*mdnsRegistrations, error) {
	ips := netinfo.LocalIPv4Strings()
	if len(ips) == 0 {
		log.Warn("mDNS unavailable: no usable IPv4 interfaces")
		return nil, nil
	}

	regs := &mdnsRegistrations{}

	// Per-device registration (always).
	if deviceName != "" {
		s, err := registerMDNS("EMOS-"+deviceName, deviceName, port, ips, txt, log)
		if err != nil {
			return nil, err
		}
		if s != nil {
			regs.servers = append(regs.servers, s)
			log.Info("mDNS announced",
				"instance", "EMOS-"+deviceName,
				"hostname", deviceName+".local",
				"port", port,
				"ips", strings.Join(ips, ","))
		}
	}

	// Shared `emos.local` shortcut (always, alongside the per-device one).
	s, err := registerMDNS(mdnsSharedInstanceLabel, mdnsSharedHost, port, ips, txt, log)
	if err != nil {
		regs.Shutdown()
		return nil, err
	}
	if s != nil {
		regs.servers = append(regs.servers, s)
		log.Info("mDNS announced",
			"instance", mdnsSharedInstanceLabel,
			"hostname", mdnsSharedHost+".local",
			"port", port,
			"ips", strings.Join(ips, ","))
	}

	return regs, nil
}

// registerMDNS wraps zeroconf.RegisterProxy
// Returns (nil, nil) when the failure is "no usable multicast interface" so
// callers can continue degraded.
func registerMDNS(instance, host string, port int, ips, txt []string, log *slog.Logger) (*zeroconf.Server, error) {
	s, err := zeroconf.RegisterProxy(
		instance,
		mdnsServiceType,
		"local.",
		port,
		host,
		ips,
		txt,
		nil, // all interfaces
	)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			log.Warn("mDNS unavailable", "instance", instance, "err", err)
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// LocalIPv4Strings re-exports netinfo.LocalIPv4Strings under the server
// package's name
func LocalIPv4Strings() []string { return netinfo.LocalIPv4Strings() }

// PreferredLANIPv4 — see LocalIPv4Strings.
func PreferredLANIPv4() string { return netinfo.PreferredLANIPv4() }
