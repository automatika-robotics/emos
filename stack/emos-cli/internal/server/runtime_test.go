package server

import (
	"errors"
	"testing"
	"time"
)

func TestRuntimeTryLockSingleSlot(t *testing.T) {
	rt := NewRuntime()
	r1 := &Run{ID: "r1", Status: RunStatusPreparing}
	if err := rt.TryLock(r1); err != nil {
		t.Fatalf("TryLock #1: %v", err)
	}
	r2 := &Run{ID: "r2", Status: RunStatusPreparing}
	if err := rt.TryLock(r2); err == nil {
		t.Fatalf("TryLock #2 should reject; current = %v", rt.Current())
	}
	// First slot still occupied by r1.
	if cur := rt.Current(); cur == nil || cur.ID != "r1" {
		t.Fatalf("Current = %v, want r1", cur)
	}
}

func TestRuntimeFailPreflightIdempotent(t *testing.T) {
	rt := NewRuntime()
	r := &Run{ID: "r", Status: RunStatusPreparing}
	if err := rt.TryLock(r); err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	rt.FailPreflight(r, errors.New("boom"))
	if r.Status != RunStatusFailed {
		t.Fatalf("Status = %q, want failed", r.Status)
	}
	if r.Error != "boom" {
		t.Fatalf("Error = %q, want boom", r.Error)
	}
	// Second call should not panic and not flip back to running.
	rt.FailPreflight(r, errors.New("again"))
	if r.Status != RunStatusFailed {
		t.Fatalf("Status changed on second FailPreflight: %q", r.Status)
	}
	if r.Error != "boom" {
		t.Fatalf("Error overwritten on second FailPreflight: %q", r.Error)
	}
	// Current slot must be released.
	if cur := rt.Current(); cur != nil {
		t.Fatalf("Current after Fail = %v, want nil", cur)
	}
}

func TestRuntimeCancelPreflightIdempotent(t *testing.T) {
	rt := NewRuntime()
	r := &Run{ID: "r", Status: RunStatusPreparing}
	if err := rt.TryLock(r); err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	rt.CancelPreflight(r)
	if r.Status != RunStatusCanceled {
		t.Fatalf("Status = %q, want canceled", r.Status)
	}
	// cancelCh should be closed; reading from a closed channel returns immediately.
	select {
	case <-r.CancelCh():
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("cancelCh not closed by CancelPreflight")
	}
	rt.CancelPreflight(r) // second call must not panic
	if cur := rt.Current(); cur != nil {
		t.Fatalf("Current after Cancel = %v, want nil", cur)
	}
}

func TestRuntimeCancelByID(t *testing.T) {
	rt := NewRuntime()
	r := &Run{ID: "abc", Status: RunStatusPreparing}
	if err := rt.TryLock(r); err != nil {
		t.Fatalf("TryLock: %v", err)
	}

	if err := rt.Cancel("not-the-id"); err == nil {
		t.Fatalf("Cancel(wrong id) should error")
	}
	if err := rt.Cancel("abc"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if r.Status != RunStatusCanceled {
		t.Fatalf("Status = %q, want canceled", r.Status)
	}
}

func TestRuntimeHistoryRotation(t *testing.T) {
	rt := NewRuntime()
	// Push 12 fails through; history caps at 10.
	for i := 0; i < 12; i++ {
		r := &Run{ID: "r", Status: RunStatusPreparing}
		if err := rt.TryLock(r); err != nil {
			t.Fatalf("TryLock #%d: %v", i, err)
		}
		rt.FailPreflight(r, errors.New("nope"))
	}
	got := rt.List()
	if len(got) != 10 {
		t.Fatalf("List() len = %d, want 10 (history cap)", len(got))
	}
	for _, run := range got {
		if run.Status != RunStatusFailed {
			t.Fatalf("history entry status = %q, want failed", run.Status)
		}
	}
}

func TestRuntimeGetReturnsCopy(t *testing.T) {
	rt := NewRuntime()
	r := &Run{ID: "a", Status: RunStatusPreparing}
	if err := rt.TryLock(r); err != nil {
		t.Fatalf("TryLock: %v", err)
	}

	if got := rt.Get("a"); got == nil || got.ID != "a" {
		t.Fatalf("Get(active) = %v, want id=a", got)
	}
	rt.FailPreflight(r, errors.New("x"))
	if got := rt.Get("a"); got == nil || got.Status != RunStatusFailed {
		t.Fatalf("Get(history) = %v, want failed", got)
	}
	if got := rt.Get("nope"); got != nil {
		t.Fatalf("Get(missing) = %v, want nil", got)
	}
}

func TestCloseOnceSafeOnNilOrClosed(t *testing.T) {
	closeOnce(nil) // must not panic

	ch := make(chan struct{})
	closeOnce(ch)
	closeOnce(ch) // second close must not panic
	select {
	case <-ch:
	default:
		t.Fatalf("channel not closed")
	}
}
