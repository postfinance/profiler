package profiler

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultProfiler(t *testing.T) {
	p := New()
	require.Equal(t, syscall.SIGHUP, p.signal)
	require.Equal(t, ":6666", p.address)
	require.Equal(t, 10*time.Minute, p.timeout)
}

func TestWithSignal(t *testing.T) {
	signal := syscall.SIGUSR2
	p := New(WithSignal(signal))
	require.Equal(t, signal, p.signal)
}

func TestWithAddress(t *testing.T) {
	address := ":8080"
	p := New(WithAddress(address))
	require.Equal(t, address, p.address)
}

func TestWithTimeout(t *testing.T) {
	timeout := 5 * time.Minute
	p := New(WithTimeout(timeout))
	require.Equal(t, timeout, p.timeout)
}
