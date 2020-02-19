package profiler_test

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"git.pnet.ch/golang/pkg/profiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	signal  = syscall.SIGUSR2
	timeout = 3 * time.Second
)

func TestMain(m *testing.M) {
	//log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func testProfiler(t *testing.T, p *profiler.Profiler) {
	p.Start()
	time.Sleep(1 * time.Second) // wait until the setup is done
	syscall.Kill(syscall.Getpid(), signal)
	assert.NoError(t, syscall.Kill(syscall.Getpid(), signal))
	time.Sleep(1 * time.Second) // wait until the signal is processed
	resp, err := http.Get(fmt.Sprintf("http://%s", p.Address()))
	assert.NoError(t, err)
	if resp != nil {
		resp.Body.Close()
	}
	p.Stop()
}

func TestStart(t *testing.T) {
	// get a free port
	l, _ := net.Listen("tcp", "")
	_, port, err := net.SplitHostPort(l.Addr().String())
	assert.NoError(t, err)
	assert.NoError(t, l.Close())
	address := fmt.Sprintf("localhost:%s", port)

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
	)
	require.NotNil(t, p)

	testProfiler(t, p)
}

func TestRestart(t *testing.T) {
	// get a free port
	l, _ := net.Listen("tcp", "")
	_, port, err := net.SplitHostPort(l.Addr().String())
	assert.NoError(t, err)
	assert.NoError(t, l.Close())
	address := fmt.Sprintf("localhost:%s", port)

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
	)
	require.NotNil(t, p)

	testProfiler(t, p)
	testProfiler(t, p)
}

type TestHookOne struct {
	PreStartupTriggered   bool
	PostShutdownTriggered bool
}

func (tho *TestHookOne) PreStart() {
	log.Println("TestHookOne PreStart triggered")
	tho.PreStartupTriggered = true
}

func (tho *TestHookOne) PostShutdown() {
	log.Println("TestHookOne PostShutdown triggered")
	tho.PostShutdownTriggered = true
}

type TestHookTwo struct {
	PreStartupTriggered   bool
	PostShutdownTriggered bool
}

func (tht *TestHookTwo) PreStart() {
	log.Println("TestHookTwo PreStart triggered")
	tht.PreStartupTriggered = true
}

func (tht *TestHookTwo) PostShutdown() {
	log.Println("TestHookTwo PostShutdown triggered")
	tht.PostShutdownTriggered = true
}

func TestWithHooks(t *testing.T) {
	// get a free port
	l, _ := net.Listen("tcp", "")
	_, port, err := net.SplitHostPort(l.Addr().String())
	assert.NoError(t, err)
	assert.NoError(t, l.Close())
	address := fmt.Sprintf("localhost:%s", port)

	one := &TestHookOne{}
	two := &TestHookTwo{}

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithHooks(one, two),
	)
	require.NotNil(t, p)

	p.Start()
	time.Sleep(1 * time.Second) // wait until the setup is done
	syscall.Kill(syscall.Getpid(), signal)
	assert.NoError(t, syscall.Kill(syscall.Getpid(), signal))
	time.Sleep(1 * time.Second) // wait until the signal is processed
	assert.True(t, one.PreStartupTriggered)
	assert.True(t, two.PreStartupTriggered)
	resp, err := http.Get(fmt.Sprintf("http://%s", p.Address()))
	assert.NoError(t, err)
	if resp != nil {
		resp.Body.Close()
	}
	p.Stop()
	assert.True(t, one.PostShutdownTriggered)
	assert.True(t, two.PostShutdownTriggered)
}

type HookFailedStart struct {
	Shutdown bool
}

func (hfs *HookFailedStart) PreStart() {
}

func (hfs *HookFailedStart) PostShutdown() {
	log.Println("HookFailedStart PostShutdown triggered")
	hfs.Shutdown = true
}

func TestFailedStart(t *testing.T) {
	t.SkipNow()
	// get a free port
	l, _ := net.Listen("tcp", "")

	// defer close of listener to get "bind: address already in use" on start
	defer l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	assert.NoError(t, err)
	address := fmt.Sprintf("localhost:%s", port)

	fh := &HookFailedStart{}
	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithHooks(fh),
	)
	require.NotNil(t, p)

	testProfiler(t, p)
	assert.True(t, fh.Shutdown)
}
