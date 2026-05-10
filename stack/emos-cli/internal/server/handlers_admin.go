package server

import (
	"net"
	"net/http"
)

// handleAdminReloadAuth refreshes the daemon's in-memory auth state from
// the on-disk config. Used by `emos config rotate-pairing` to push the
// just-written pairing hash into the running daemon without requiring a
// full systemd restart.
//
// Loopback-only: the only legitimate caller is the local CLI.
func (s *Server) handleAdminReloadAuth(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeErr(w, http.StatusForbidden, codeForbidden, "admin endpoint is loopback-only")
		return
	}
	if err := s.auth.Reload(); err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// isLoopbackRequest reports whether the request originated on the same
// host.
func isLoopbackRequest(r *http.Request) bool {
	for _, h := range []string{
		"X-Forwarded-For",
		"X-Real-IP",
		"True-Client-IP",
		"X-Cluster-Client-IP",
		"Forwarded",
	} {
		if r.Header.Get(h) != "" {
			return false
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
