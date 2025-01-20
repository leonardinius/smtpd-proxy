package systemtest

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/leonardinius/smtpd-proxy/app/cmd"
	"github.com/stretchr/testify/require"
)

// BindHost host to bind to in local smoke tests.
const BindHost = "127.0.0.1"

// RunMainWithConfig run app in test suite.
func RunMainWithConfig(ctx context.Context, t *testing.T, yamlConfig string, port int, test func(t *testing.T, conn net.Conn)) {
	t.Helper()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		cfg *os.File
		err error
	)
	if cfg, err = createConfigurationFle(t.TempDir(), yamlConfig); err != nil {
		t.Fatal("Failed to create temporary comfiguration file", err)
	}

	go func() {
		_ = cmd.Main(ctx, "--verbose", "-c", cfg.Name())
		cancel()
	}()

	conn := waitForPortListenStart(ctx, t, port)
	err = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	require.NoError(t, err, "SMTP set connection deadline error")
	test(t, conn)
}

func waitForPortListenStart(ctx context.Context, t *testing.T, port int) (conn net.Conn) {
	t.Helper()

	addr := net.JoinHostPort(BindHost, strconv.Itoa(port))

	poll := time.NewTicker(50 * time.Millisecond)
	defer poll.Stop()

	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			t.Fatalf("%s port open timeout", addr)

		case <-ctx.Done():
			t.Fatalf("%s port open error, parent context is done: %v", addr, ctx.Err())

		case <-poll.C:
			var d net.Dialer
			conn, _ = checkAddr(ctx, &d, addr)
			if conn != nil {
				return conn
			}
		}
	}
}

func checkAddr(ctx context.Context, d *net.Dialer, addr string) (net.Conn, error) {
	limitCtx, limitCancelFn := context.WithTimeout(ctx, 50*time.Millisecond)
	defer limitCancelFn()
	return d.DialContext(limitCtx, "tcp", addr)
}

func createConfigurationFle(tempdir, content string) (tmpFile *os.File, err error) {
	tmpFile, err = os.CreateTemp(tempdir, "smtpd-proxy-*-test.yml")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}

	text := []byte(content)
	if _, err = tmpFile.Write(text); err != nil {
		log.Fatal("Failed to write to temporary file", err)
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		log.Fatal(err)
	}
	return
}

var acquiredPorts = new(sync.Map)

// DynamicPort supplies random free net ports to use.
func DynamicPort() int {
	port := dynamicPort()
	for {
		if _, loaded := acquiredPorts.LoadOrStore(port, true); !loaded {
			break
		}
		port = dynamicPort()
	}
	return port
}

func dynamicPort() int {
	listener, err := net.Listen("tcp", BindHost+":0")
	if err != nil {
		panic(err)
	}
	defer func() {
		err := listener.Close()
		if err != nil {
			panic(err)
		}
	}()

	if port, ok := listener.Addr().(*net.TCPAddr); ok {
		return port.Port
	}

	panic("Failed to get port")
}
