package systemtest

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"testing"

	"github.com/leonardinius/smtpd-proxy/app/upstream/forwarder"
	"github.com/stretchr/testify/assert"
)

var reciepientsTo = []string{"recipient@example.net"}

var messageBody string = strings.Join([]string{
	"To: <discard-simple-smtp@tld.invalid>",
	"From: <gotest-simple-smtp@esmtp.email>",
	"Subject: Test E-mail!",
	"",
	"This is the email body (SMTP).",
	"",
}, "\r\n")

func TestSmokeAuthCredentials(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	port := DynamicPort()
	proxyEndpoint := fmt.Sprintf("%s:%d", BindHost, port)
	config := fmt.Sprintf(`
smtpd-proxy:
  listen: %s
  ehlo: localhost
  username: user@example.com
  password: password
  is_anon_auth_allowed: false
  upstream-servers:
  - type: log
`, proxyEndpoint)
	RunMainWithConfig(t, ctx, config, port, func(t *testing.T, conn net.Conn) {
		credentials := []struct {
			name string
			auth smtp.Auth
			err  string
		}{
			{"plain-ok", smtp.PlainAuth("", "user@example.com", "password", BindHost), ""},
			{"plain-wrong-host", smtp.PlainAuth("", "user@example.com", "password", "wrong-host"), "wrong host name"},
			{"plain-wrong-user", smtp.PlainAuth("", "wrong@example.com", "password", BindHost), "invalid username or password"},
			{"plain-wrong-password", smtp.PlainAuth("", "user@example.com", "wrong-password", BindHost), "invalid username or password"},
			{"plain-wrong-identity", smtp.PlainAuth("wrong-identity", "user@example.com", "password", BindHost), "Identities not supported"},
			//
			{"login-ok", forwarder.NewLoginAuth("user@example.com", "password", BindHost), ""},
			{"login-wrong-host", forwarder.NewLoginAuth("user@example.com", "password", "wrong-host"), "wrong host name"},
			{"login-wrong-user", forwarder.NewLoginAuth("wrong@example.com", "password", BindHost), "invalid username or password"},
			{"login-wrong-password", forwarder.NewLoginAuth("user@example.com", "wrong-password", BindHost), "invalid username or password"},
			//
			{"anon-fail", nil, "anonymous access is not allowed"},
		}

		for _, test := range credentials {
			t.Run(test.name, func(t *testing.T) {
				err := smtp.SendMail(proxyEndpoint, test.auth, "sender@example.org", reciepientsTo, []byte(messageBody))
				if test.err == "" {
					assert.NoError(t, err)
				} else {
					assert.ErrorContains(t, err, test.err)
				}
			})
		}

	})
}

func TestSmokeAnonCredentialsOk(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	port := DynamicPort()
	proxyEndpoint := fmt.Sprintf("%s:%d", BindHost, port)
	config := fmt.Sprintf(`
smtpd-proxy:
  listen: %s
  ehlo: localhost
  is_anon_auth_allowed: true
  upstream-servers:
  - type: log
`, proxyEndpoint)
	RunMainWithConfig(t, ctx, config, port, func(t *testing.T, conn net.Conn) {
		err := smtp.SendMail(proxyEndpoint, nil, "sender@example.org", reciepientsTo, []byte(messageBody))
		assert.NoError(t, err)
	})
}
