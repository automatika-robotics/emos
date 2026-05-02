package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"time"
)

// tailLog opens a log file and streams lines through `out` until the context
// cancels or `done` closes. It first replays the file from the start, then
// polls for newly-appended lines (250ms cadence). Designed for run logs that
// strictly grow; rotation isn't expected.
func tailLog(ctx context.Context, path string, done <-chan struct{}, out chan<- string) error {
	// Wait briefly for the file to appear (the recipe may take a moment to
	// open it) before giving up.
	deadline := time.Now().Add(3 * time.Second)
	var f *os.File
	var err error
	for {
		f, err = os.Open(path)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err == nil {
			select {
			case out <- trimNewline(line):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		if err != io.EOF {
			return err
		}
		// EOF — wait for more data, unless the run is finished.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			// Drain any final partial line and exit.
			if line != "" {
				select {
				case out <- trimNewline(line):
				default:
				}
			}
			return nil
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func trimNewline(s string) string {
	if n := len(s); n > 0 && s[n-1] == '\n' {
		return s[:n-1]
	}
	return s
}
