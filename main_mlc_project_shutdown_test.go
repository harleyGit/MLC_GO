//go:build production

package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHGWaitServeErrorsCollectsFailures(t *testing.T) {
	serveErr := make(chan error, 2)
	serveErr <- nil
	serveErr <- errors.New("realtime failed")
	if err := hgWaitServeErrors(serveErr, 2, time.Second); err == nil || !strings.Contains(err.Error(), "realtime failed") {
		t.Fatalf("hgWaitServeErrors() error = %v", err)
	}
}

func TestHGWaitServeErrorsHasBoundedTimeout(t *testing.T) {
	serveErr := make(chan error)
	startedAt := time.Now()
	err := hgWaitServeErrors(serveErr, 2, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "2 server listeners did not stop") {
		t.Fatalf("hgWaitServeErrors() timeout error = %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("hgWaitServeErrors() blocked for %s", elapsed)
	}
}
