package proc

import (
	"os"
	"testing"
)

func TestGetStartTime_Self(t *testing.T) {
	pid := os.Getpid()
	st := GetStartTime(pid)
	if st == "" {
		t.Fatal("expected non-empty start time for own process")
	}
	// Calling again should return the same value.
	st2 := GetStartTime(pid)
	if st != st2 {
		t.Errorf("start time changed: %q vs %q", st, st2)
	}
}

func TestGetStartTime_NonExistent(t *testing.T) {
	// PID 99999999 is unlikely to exist.
	st := GetStartTime(99999999)
	if st != "" {
		t.Errorf("expected empty start time for non-existent PID, got %q", st)
	}
}

func TestIsAlive_Self(t *testing.T) {
	pid := os.Getpid()
	st := GetStartTime(pid)
	if !IsAlive(pid, st) {
		t.Error("expected own process to be alive")
	}
}

func TestIsAlive_NonExistent(t *testing.T) {
	if IsAlive(99999999, "") {
		t.Error("expected non-existent PID to not be alive")
	}
}

func TestIsAlive_WrongStartTime(t *testing.T) {
	pid := os.Getpid()
	// Use a fake start time — should detect PID reuse.
	if IsAlive(pid, "Thu Jan  1 00:00:00 1970") {
		t.Error("expected IsAlive to return false for wrong start time")
	}
}

func TestIsAlive_ZeroPID(t *testing.T) {
	if IsAlive(0, "") {
		t.Error("expected PID 0 to not be alive")
	}
}

func TestIsAlive_NegativePID(t *testing.T) {
	if IsAlive(-1, "") {
		t.Error("expected negative PID to not be alive")
	}
}
