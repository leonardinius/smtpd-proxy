package systemtest

import (
	"context"
	"sync"
	"testing"

	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/leonardinius/smtpd-proxy/app/cmd"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
)

// BindHost host to bind to in local smoke tests
const BindHost = "127.0.0.1"

// RunMainWithConfig run app in test suite
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
	test(t, conn)
}

func waitForPortListenStart(ctx context.Context, t *testing.T, port int) (conn net.Conn) {
	var d net.Dialer
	var err error
	addr := fmt.Sprintf("%s:%d", BindHost, port)
	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	select {
	case <-poll.C:
		conn, _ = checkAddr(ctx, &d, addr)
		if conn != nil {
			break
		}
	case <-timeout.C:
		t.Fatal("SMTP open timeout")
		break
	}

	require.NotNil(t, conn)
	err = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	if err != nil {
		t.Fatal("SMTP set connection deadline error", err)
	}
	return conn
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

// DynamicPort supplies random free net ports to use
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
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:0", BindHost))
	if err != nil {
		panic(err)
	}
	defer func() {
		err := listener.Close()
		if err != nil {
			panic(err)
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	return port
}

// TerminateContainer terminates container if present
func TerminateContainer(ctx context.Context, container tc.Container) error {
	if container != nil {
		return container.Terminate(ctx)
	}
	return nil
}
