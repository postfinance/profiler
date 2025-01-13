package profiler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/postfinance/profiler"
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

func testEventHandler(w io.Writer, mu *sync.Mutex) profiler.EventHandler {
	l := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return func(eventType profiler.EventType, msg string, args ...any) {
		mu.Lock()
		defer mu.Unlock()

		switch eventType {
		case profiler.InfoEvent:
			l.Info(msg, args...)
		case profiler.ErrorEvent:
			l.Error(msg, args...)
		}
	}
}

func testProfiler(t *testing.T,
	p *profiler.Profiler,
	ep string,
	success bool,
	checkBody func(t *testing.T, body []byte),
) {
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)

	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.NoError(t, syscall.Kill(syscall.Getpid(), signal))

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
	require.Equal(t, success, err == nil, err)

	if resp != nil && resp.Body != nil {
		var buf bytes.Buffer

		buf.ReadFrom(resp.Body)
		_ = resp.Body.Close()

		checkBody(t, buf.Bytes())
	}

	cancel()
}

func TestStart(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	address := testAddress(t)

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithEventHandler(testEventHandler(&buf, &mu)),
	)
	require.NotNil(t, p)

	testProfiler(t, p, "", true, nil)

	time.Sleep(100 * time.Millisecond) // switch goroutine
	mu.Lock()
	t.Logf("\n%s", buf.String())
	mu.Unlock()
}

func TestRestart(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	address := testAddress(t)

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithEventHandler(testEventHandler(&buf, &mu)),
	)
	require.NotNil(t, p)

	testProfiler(t, p, "", true, nil)
	time.Sleep(100 * time.Millisecond) // switch goroutine
	testProfiler(t, p, "", true, nil)

	time.Sleep(100 * time.Millisecond) // switch goroutine
	mu.Lock()
	t.Logf("\n%s", buf.String())
	mu.Unlock()
}

func TestMultipleStartStop(t *testing.T) {
	address := testAddress(t)

	startSignalHandlerEvents := new(atomic.Int32)
	stopSignalHandlerEvents := new(atomic.Int32)

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithEventHandler(func(_ profiler.EventType, msg string, _ ...any) {
			if strings.Contains(msg, "start profiler signal handler") {
				startSignalHandlerEvents.Add(1)
			}
			if strings.Contains(msg, "stop profiler signal handler") {
				stopSignalHandlerEvents.Add(1)
			}
		}),
	)
	require.NotNil(t, p)

	ctx1, cancel1 := context.WithCancel(context.Background())

	p.Start(ctx1)
	p.Start(ctx1)
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.Equal(t, int32(1), startSignalHandlerEvents.Load())
	require.Equal(t, int32(0), stopSignalHandlerEvents.Load())

	cancel1()
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.Equal(t, int32(1), startSignalHandlerEvents.Load())
	require.Equal(t, int32(1), stopSignalHandlerEvents.Load())

	ctx2, cancel2 := context.WithCancel(context.Background())

	p.Start(ctx2)
	p.Start(ctx2)
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.Equal(t, int32(2), startSignalHandlerEvents.Load())
	require.Equal(t, int32(1), stopSignalHandlerEvents.Load())

	cancel2()
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.Equal(t, int32(2), startSignalHandlerEvents.Load())
	require.Equal(t, int32(2), stopSignalHandlerEvents.Load())
}

func TestExpvars(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	hello := expvar.NewString("hello")
	hello.Set("world")

	address := testAddress(t)

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithEventHandler(testEventHandler(&buf, &mu)),
	)
	require.NotNil(t, p)

	testProfiler(t, p, "/debug/vars", true, func(t *testing.T, body []byte) {
		m := make(map[string]any)
		require.NoError(t, json.Unmarshal(body, &m))
		require.Equal(t, "world", m["hello"].(string))
		t.Log("hello", m["hello"].(string))
	})

	time.Sleep(100 * time.Millisecond) // switch goroutine
	mu.Lock()
	t.Logf("\n%s", buf.String())
	mu.Unlock()
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
	var buf bytes.Buffer
	var mu sync.Mutex

	address := testAddress(t)

	one := &TestHookOne{}
	two := &TestHookTwo{}

	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithEventHandler(testEventHandler(&buf, &mu)),
		profiler.WithHooks(one, two),
	)
	require.NotNil(t, p)

	ctx, cancel := context.WithCancel(context.Background())

	p.Start(ctx)
	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.NoError(t, syscall.Kill(syscall.Getpid(), signal))

	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.True(t, one.HasPreStartupTriggered())
	require.True(t, two.HasPreStartupTriggered())

	resp, err := http.Get(fmt.Sprintf("http://%s", p.Address()))
	require.NoError(t, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	cancel()

	time.Sleep(100 * time.Millisecond) // switch goroutine

	require.True(t, one.HasPostShutdownTriggered())
	require.True(t, two.HasPostShutdownTriggered())

	time.Sleep(100 * time.Millisecond) // switch goroutine
	mu.Lock()
	t.Logf("\n%s", buf.String())
	mu.Unlock()
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
	var buf bytes.Buffer
	var mu sync.Mutex

	// get a free port
	l, _ := net.Listen("tcp", "")

	// defer close of listener to get "bind: address already in use" on start
	defer l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)

	address := fmt.Sprintf("localhost:%s", port)

	fh := &HookFailedStart{}
	p := profiler.New(
		profiler.WithSignal(signal),
		profiler.WithAddress(address),
		profiler.WithTimeout(timeout),
		profiler.WithEventHandler(testEventHandler(&buf, &mu)),
		profiler.WithHooks(fh),
	)
	require.NotNil(t, p)

	testProfiler(t, p, "", false, nil)
	require.True(t, fh.IsShutdown())

	time.Sleep(100 * time.Millisecond) // switch goroutine
	mu.Lock()
	t.Logf("\n%s", buf.String())
	mu.Unlock()
}
