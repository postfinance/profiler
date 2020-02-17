package profiler

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultProfiler(t *testing.T) {
	p := New()
	assert.Equal(t, syscall.SIGUSR1, p.sig)
	assert.Equal(t, ":6666", p.addr)
	assert.Equal(t, 10*time.Minute, p.timeout)
}

func TestWithSignal(t *testing.T) {
	sig := syscall.SIGUSR2
	p := New(WithSignal(sig))
	assert.Equal(t, sig, p.sig)
}

func TestWithAddress(t *testing.T) {
	addr := ":8080"
	p := New(WithAddress(addr))
	assert.Equal(t, addr, p.addr)
}

func TestWithTimeout(t *testing.T) {
	timeout := 5 * time.Minute
	p := New(WithTimeout(timeout))
	assert.Equal(t, timeout, p.timeout)
}

func TestStart(t *testing.T) {
	sig := syscall.SIGUSR2
	timeout := 3 * time.Second

	l, _ := net.Listen("tcp", "")
	_, port, err := net.SplitHostPort(l.Addr().String())
	assert.NoError(t, err)
	err = l.Close()
	assert.NoError(t, err)
	addr := fmt.Sprintf("localhost:%s", port)

	t.Log("listener address:", addr)
	prof := New(
		WithSignal(sig),
		WithAddress(addr),
		WithTimeout(timeout),
	)
	require.NotNil(t, prof)
	prof.Start()
	time.Sleep(1 * time.Second) // wait until the setup is done
	syscall.Kill(syscall.Getpid(), sig)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second) // wait until the signal is processed
	resp, err := http.Get(fmt.Sprintf("http://%s", addr))
	assert.NoError(t, err)
	if resp != nil {
		resp.Body.Close()
	}
}
