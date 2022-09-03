package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const bindHost = "127.0.0.1"

func Test_Main(t *testing.T) {
	port := dynamicPort()
	yamlConfig := fmt.Sprintf(`
smtpd-proxy:
  listen: %s:%d
  ehlo: localhost
  username: user
  password: secret
  is_anon_auth_allowed: true
  upstream-servers:
    - type: log
      weight: 70	  
`, bindHost, port)

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
		os.Args = []string{"test", "-c", cfg.Name()}
		main()
		close(finished)
	}()

	// defer cleanup because require check below can fail
	defer func() {
		close(done)
		<-finished
	}()

	{
		conn := waitForPortListenStart(t, port)
		defer conn.Close()

		bufReader := bufio.NewReader(conn)
		if _, err := conn.Write([]byte("EHLO test\r\n")); err != nil {
			t.Errorf("Failed to EHLO, %s", err)
		}

		response := readStrings(bufReader)
		for _, s := range [...]string{
			"220 localhost ESMTP Service Ready",
			"250-Hello test",
			"250-PIPELINING",
			"250-8BITMIME",
			"250-ENHANCEDSTATUSCODES",
			"250-CHUNKING",
			"250-SMTPUTF8",
		} {
			require.Contains(t, response, s)
		}

		for _, s := range response {
			if strings.Contains(s, "250-AUTH") {
				for _, auth := range [...]string{
					"PLAIN", "LOGIN",
				} {
					require.Contains(t, s, auth)
				}
			}
		}
	}
}

func waitForPortListenStart(t *testing.T, port int) (conn net.Conn) {
	var d net.Dialer
	var err error
	addr := fmt.Sprintf("%s:%d", bindHost, port)
	ctx, cancelFn := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelFn()
	poll := time.Tick(10 * time.Millisecond)
	select {
	case <-poll:
		limitCtx, limitCancelFn := context.WithTimeout(context.Background(), 10*time.Millisecond)
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

func readStrings(b *bufio.Reader) []string {
	out := make([]string, 0)
	for {
		responseString, err := b.ReadString('\n')
		if err != nil {
			break
		}
		responseString = strings.TrimRight(responseString, "\r\n")
		out = append(out, responseString)
		log.Printf("[TEST] >> %s\n", responseString)
	}

	return out
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
func dynamicPort() int {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:0", bindHost))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	return port
}
