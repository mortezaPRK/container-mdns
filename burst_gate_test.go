package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewBurstGate_Debounce(t *testing.T) {
	req := require.New(t)
	in := make(chan nothing)
	out := newBurstGate(50*time.Millisecond, in)

	// Send 3 events in quick succession
	go func() {
		in <- nothing{}
		in <- nothing{}
		in <- nothing{}
		close(in)
	}()

	var count int
	for range out {
		count++
	}

	req.Equal(1, count, "Expected 1 output for burst")
}

func TestNewBurstGate_MultipleBursts(t *testing.T) {
	req := require.New(t)
	in := make(chan nothing)
	out := newBurstGate(30*time.Millisecond, in)

	go func() {
		in <- nothing{}
		in <- nothing{}
		time.Sleep(50 * time.Millisecond)
		in <- nothing{}
		close(in)
	}()

	var count int
	for range out {
		count++
	}

	req.Equal(2, count, "Expected 2 outputs for two bursts")
}
