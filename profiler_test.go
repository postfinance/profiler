package profiler_test

import (
	"bytes"
	"encoding/json"
	"expvar"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/postfinance/profiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nolint: gochecknoglobals
var (
	signal  = syscall.SIGUSR2
	timeout = 3 * time.Second
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func testAddress(t *testing.T) string {
	// get a free port
	l, _ := net.Listen("tcp", "")
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	require.NoError(t, l.Close())

	return fmt.Sprintf("localhost:%s", port)
}

func testProfiler(t *testing.T,
	p *profiler.Profiler,
	ep string,
	success bool,
	checkBody func(t *testing.T, body []byte),
) {
	p.Start()
	time.Sleep(100 * time.Millisecond) // switch goroutine
	assert.NoError(t, syscall.Kill(syscall.Getpid(), signal))
	time.Sleep(100 * time.Millisecond) // switch goroutine

	client := http.Client{
		Timeout: 10 * time.Millisecond,
	}

	if ep == "" {
		ep = "/debug/pprof"

		if checkBody == nil {
			checkBody = func(t *testing.T, b []byte) {
				require.Contains(t, string(b), "<title>/debug/pprof/</title>")
			}
		}
	}

	resp, err := client.Get(fmt.Sprintf("http://%s%s", p.Address(), ep))
	assert.Equal(t, err == nil, success)

	if resp != nil && resp.Body != nil {
		var buf bytes.Buffer

		buf.ReadFrom(resp.Body)
		_ = resp.Body.Close()

		checkBody(t, buf.Bytes())
	}

	p.Stop()
}

func TestStart(t *testing.T) {
	address := testAddress(t)

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
	)
	require.NotNil(t, p)

	testProfiler(t, p, "", true, nil)
}

func TestRestart(t *testing.T) {
	address := testAddress(t)

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
	)
	require.NotNil(t, p)

	testProfiler(t, p, "", true, nil)
	testProfiler(t, p, "", true, nil)
}

func TestFastRestart(t *testing.T) {
	address := testAddress(t)

	startEvent := 0
	stopEvent := 0

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithEventHandler(func(msg string, args ...any) {
			if strings.Contains(msg, "start profiler signal handler") {
				startEvent++
			}
			if strings.Contains(msg, "stop profiler signal handler") {
				stopEvent++
			}
		}),
	)
	require.NotNil(t, p)

	p.Start()
	p.Stop()
	p.Start()
	p.Stop()
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.Equal(t, 2, startEvent)
	require.Equal(t, 2, stopEvent)
}

func TestMultipleStartStop(t *testing.T) {
	address := testAddress(t)

	startEvent := 0
	stopEvent := 0

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithEventHandler(func(msg string, args ...any) {
			if strings.Contains(msg, "start profiler signal handler") {
				startEvent++
			}
			if strings.Contains(msg, "stop profiler signal handler") {
				stopEvent++
			}
		}),
	)
	require.NotNil(t, p)

	p.Start()
	p.Start()
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.Equal(t, 1, startEvent)
	require.Equal(t, 0, stopEvent)

	p.Stop()
	p.Stop()
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.Equal(t, 1, startEvent)
	require.Equal(t, 1, stopEvent)

	p.Start()
	p.Start()
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.Equal(t, 2, startEvent)
	require.Equal(t, 1, stopEvent)

	p.Stop()
	p.Stop()
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.Equal(t, 2, startEvent)
	require.Equal(t, 2, stopEvent)
}

func TestExpvars(t *testing.T) {
	hello := expvar.NewString("hello")
	hello.Set("world")

	address := testAddress(t)

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
	)
	require.NotNil(t, p)

	testProfiler(t, p, "/debug/vars", true, func(t *testing.T, body []byte) {
		m := make(map[string]any)
		require.NoError(t, json.Unmarshal(body, &m))
		require.Equal(t, "world", m["hello"].(string))
		t.Log("hello", m["hello"].(string))
	})
}

// =============================================================================

type TestHookOne struct {
	sync.Mutex
	PreStartupTriggered   bool
	PostShutdownTriggered bool
}

func (tho *TestHookOne) PreStart() {
	log.Println("TestHookOne PreStart triggered")
	tho.Lock()
	defer tho.Unlock()
	tho.PreStartupTriggered = true
}

func (tho *TestHookOne) HasPreStartupTriggered() bool {
	tho.Lock()
	defer tho.Unlock()

	return tho.PreStartupTriggered
}

func (tho *TestHookOne) PostShutdown() {
	log.Println("TestHookOne PostShutdown triggered")
	tho.Lock()
	defer tho.Unlock()
	tho.PostShutdownTriggered = true
}

func (tho *TestHookOne) HasPostShutdownTriggered() bool {
	tho.Lock()
	defer tho.Unlock()

	return tho.PostShutdownTriggered
}

type TestHookTwo struct {
	sync.Mutex
	PreStartupTriggered   bool
	PostShutdownTriggered bool
}

func (tht *TestHookTwo) PreStart() {
	log.Println("TestHookTwo PreStart triggered")
	tht.Lock()
	defer tht.Unlock()
	tht.PreStartupTriggered = true
}

func (tht *TestHookTwo) HasPreStartupTriggered() bool {
	tht.Lock()
	defer tht.Unlock()

	return tht.PreStartupTriggered
}

func (tht *TestHookTwo) PostShutdown() {
	log.Println("TestHookTwo PostShutdown triggered")
	tht.Lock()
	defer tht.Unlock()
	tht.PostShutdownTriggered = true
}

func (tht *TestHookTwo) HasPostShutdownTriggered() bool {
	tht.Lock()
	defer tht.Unlock()

	return tht.PostShutdownTriggered
}

func TestWithHooks(t *testing.T) {
	address := testAddress(t)

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
	time.Sleep(100 * time.Millisecond) // switch goroutine
	assert.NoError(t, syscall.Kill(syscall.Getpid(), signal))
	time.Sleep(100 * time.Millisecond) // switch goroutine
	assert.True(t, one.HasPreStartupTriggered())
	assert.True(t, two.HasPreStartupTriggered())

	resp, err := http.Get(fmt.Sprintf("http://%s", p.Address()))
	assert.NoError(t, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	p.Stop()
	assert.True(t, one.HasPostShutdownTriggered())
	assert.True(t, two.HasPostShutdownTriggered())
}

// =============================================================================

type HookFailedStart struct {
	sync.Mutex
	Shutdown bool
}

func (hfs *HookFailedStart) PreStart() {
	log.Println("HookFailedStart PreStart triggered")
}

func (hfs *HookFailedStart) PostShutdown() {
	log.Println("HookFailedStart PostShutdown triggered")
	hfs.Lock()
	hfs.Shutdown = true
	hfs.Unlock()
}

func (hfs *HookFailedStart) IsShutdown() bool {
	hfs.Lock()
	defer hfs.Unlock()

	return hfs.Shutdown
}
func TestFailedStart(t *testing.T) {
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

	testProfiler(t, p, "", false, nil)
	assert.True(t, fh.IsShutdown())
}
