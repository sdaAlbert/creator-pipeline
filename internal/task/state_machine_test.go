package task

import "testing"

func TestCanTransition(t *testing.T) {
	if !CanTransition(StatusPending, StatusRunning) {
		t.Fatal("pending should transition to running")
	}
	if CanTransition(StatusSucceeded, StatusPending) {
		t.Fatal("succeeded should be terminal")
	}
}
