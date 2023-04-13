package systemtest

import (
	"context"
	"testing"

	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/leonardinius/smtpd-proxy/app/cmd"
	"github.com/leonardinius/smtpd-proxy/app/zlog"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
)

// BindHost host to bind to in local smoke tests
const BindHost = "127.0.0.1"

// RunMainWithConfig run app in test suite
func RunMainWithConfig(t *testing.T, yamlConfig string, port int, test func(t *testing.T, conn net.Conn)) {
	var (
		cfg *os.File
		err error
	)
	if cfg, err = createConfigurationFle(yamlConfig); err != nil {
		t.Fatal("Failed to create temporary comfiguration file", err)
	}
	// comment this out to troubleshoot if the test fails
	defer os.Remove(cfg.Name())

	serverCh := make(chan cmd.ServerSignal)
	done := make(chan struct{})
	go func() {
		<-done
		serverCh <- cmd.ServerStopSignal
	}()

	finished := make(chan struct{})
	go func() {
		cmd.Main(serverCh, "--verbose", "-c", cfg.Name())
		close(finished)
	}()

	// defer cleanup because require check below can fail
	defer func() {
		close(done)
		<-finished
	}()

	conn := waitForPortListenStart(t, port)
	defer func() {
		err = conn.Close()
		zlog.Debugf("conn.Close() error: %v", err)
	}()

	test(t, conn)
}

func waitForPortListenStart(t *testing.T, port int) (conn net.Conn) {
	var d net.Dialer
	var err error

	addr := fmt.Sprintf("%s:%d", BindHost, port)
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()
	select {
	case <-poll.C:
		conn = checkAddr(&d, addr)
		if conn != nil {
			break
		}
	case <-ctx.Done():
		if ctx.Err() != nil {
			t.Fatal("SMTP open error", ctx.Err())
		}
		break
	}

	require.NotNil(t, conn)
	err = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	if err != nil {
		t.Fatal("SMTP open error", err)
	}
	return conn
}

func checkAddr(d *net.Dialer, addr string) net.Conn {
	limitCtx, limitCancelFn := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer limitCancelFn()
	if conn, err := d.DialContext(limitCtx, "tcp", addr); err == nil {
		return conn
	}
	return nil
}

func createConfigurationFle(content string) (tmpFile *os.File, err error) {
	tmpFile, err = os.CreateTemp(os.TempDir(), "smtpd-proxy-*-test.yml")
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

// DynamicPort supplies random free net ports to use
func DynamicPort() int {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:0", BindHost))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
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
