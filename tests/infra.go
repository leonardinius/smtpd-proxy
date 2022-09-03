package systemtest

import (
	"context"
	"testing"

	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/leonardinius/smtpd-proxy/app/cmd"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
)

// BindHost host to bind to in local smoke tests
const BindHost = "127.0.0.1"

// RunMainWithConfig run aopp in test suite
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

	done := make(chan struct{})
	go func() {
		<-done
		e := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		require.NoError(t, e)
	}()

	finished := make(chan struct{})
	go func() {
		os.Args = []string{"test", "--verbose", "-c", cfg.Name()}
		cmd.Main()
		close(finished)
	}()

	// defer cleanup because require check below can fail
	defer func() {
		close(done)
		<-finished
	}()

	conn := waitForPortListenStart(t, port)
	defer conn.Close()
	test(t, conn)
}

func waitForPortListenStart(t *testing.T, port int) (conn net.Conn) {
	var d net.Dialer
	var err error

	addr := fmt.Sprintf("%s:%d", BindHost, port)
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	select {
	case <-poll.C:
		limitCtx, limitCancelFn := context.WithTimeout(context.Background(), 5*time.Second)
		defer limitCancelFn()
		conn, err = d.DialContext(limitCtx, "tcp", addr)
		if err == nil {
			break
		}
		conn = nil
	case <-ctx.Done():
		if ctx.Err() != nil {
			t.Fatal("SMTP open error", ctx.Err())
		}
		break
	}

	require.NotNil(t, conn)
	err = conn.SetDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		t.Fatal("SMTP open error", err)
	}
	return conn
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
