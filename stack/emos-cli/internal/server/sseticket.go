package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// An authenticated POST mints a short-lived single-use ticket;
// the SSE URL carries the ticket instead of the long-lived token.
// Tickets live in memory only and are consumed the first time they're presented;
// replay attempts fail.
type sseTicketStore struct {
	mu      sync.Mutex
	live    map[string]time.Time // ticket -> expiry
	ttl     time.Duration
	maxLive int
}

func newSSETicketStore() *sseTicketStore {
	return &sseTicketStore{
		live:    make(map[string]time.Time),
		ttl:     30 * time.Second,
		maxLive: 1024,
	}
}

// Issue returns a new opaque ticket and the expiry instant.
func (s *sseTicketStore) Issue() (string, time.Time, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", time.Time{}, err
	}
	ticket := hex.EncodeToString(b[:])
	exp := time.Now().Add(s.ttl)

	s.mu.Lock()
	defer s.mu.Unlock()
	// Cheap GC of any expired tickets so the map can't grow unbounded if
	// a tab opens many SSE streams without consuming them.
	s.gcLocked()
	if len(s.live) >= s.maxLive {
		// Refuse rather than evict. Silent eviction would race with a
		// legitimate SSE open and produce confusing 401s.
		return "", time.Time{}, errors.New("too many in-flight tickets")
	}
	s.live[ticket] = exp
	return ticket, exp, nil
}

// Consume returns true exactly once per ticket: on the first call after
// Issue, before expiry. Subsequent calls (or calls after expiry) return
// false. Idempotent for the second-call case.
func (s *sseTicketStore) Consume(ticket string) bool {
	if ticket == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.live[ticket]
	if !ok {
		return false
	}
	delete(s.live, ticket)
	if time.Now().After(exp) {
		return false
	}
	return true
}

// gcLocked drops expired tickets. Caller holds s.mu.
func (s *sseTicketStore) gcLocked() {
	now := time.Now()
	for k, exp := range s.live {
		if now.After(exp) {
			delete(s.live, k)
		}
	}
}
