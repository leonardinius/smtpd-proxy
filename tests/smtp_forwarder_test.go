package systemtest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var _ suite.SetupAllSuite = (*SMTPSystemTestSuite)(nil)
var _ suite.TearDownAllSuite = (*SMTPSystemTestSuite)(nil)

// SMTPSystemTestSuite suite
type SMTPSystemTestSuite struct {
	suite.Suite
	ctx   context.Context
	smtpd tc.Container
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSMTPSystemTestSuite(t *testing.T) {
	suite.Run(t, new(SMTPSystemTestSuite))
}

func (su *SMTPSystemTestSuite) SetupSuite() {
	var err error
	su.ctx = context.Background()
	su.smtpd, err = initFakeSMTPContainer(su.ctx)
	if err != nil {
		su.T().Fatalf("Errors: %v ", err)
	}
}

func (su *SMTPSystemTestSuite) TearDownSuite() {
	err := TerminateContainer(su.ctx, su.smtpd)
	require.NoError(su.T(), err)
	su.smtpd = nil
}

func (su *SMTPSystemTestSuite) TestSmokeSMTPForwardSimpleEmail() {
	port := DynamicPort()
	proxyEndpoint := fmt.Sprintf("%s:%d", BindHost, port)
	smtpHost, err := su.smtpd.Host(su.ctx)
	require.NoError(su.T(), err)
	_smtpPort, err := su.smtpd.MappedPort(su.ctx, "5025/tcp")
	require.NoError(su.T(), err)
	smtpPort := strings.SplitN(string(_smtpPort), "/", 2)[0]
	apiEndpoint, err := su.smtpd.PortEndpoint(su.ctx, "5080/tcp", "http")
	require.NoError(su.T(), err)
	config := fmt.Sprintf(`
smtpd-proxy:
  listen: %s
  ehlo: localhost
  username: user@example.com
  password: password
  is_anon_auth_allowed: false
  upstream-servers:
  - type: smtp
    settings:
      addr: %s:%s
      host: %s
      auth: anon
`, proxyEndpoint, smtpHost, smtpPort, smtpHost)
	RunMainWithConfig(su.T(), config, port, func(t *testing.T, conn net.Conn) {
		fromEmail := "<gotest-simple-smtp@esmtp.email>"
		// Setup authentication information.
		auth := smtp.PlainAuth("", "user@example.com", "password", BindHost)
		to := []string{"recipient@example.net"}
		msg := strings.Join([]string{
			"To: <discard-simple-smtp@tld.invalid>",
			"From: " + fromEmail,
			"Subject: Test E-mail!",
			"",
			"This is the email body (SMTP).",
			"",
		}, "\r\n")
		err := smtp.SendMail(proxyEndpoint, auth, "sender@example.org", to, []byte(msg))
		require.NoError(t, err)

		smtpFakerReceived := requireFakerReceivedEmailWithContains(t, apiEndpoint, "gotest-simple-smtp@esmtp.email")
		assert.Contains(t, smtpFakerReceived.RawData,
			"This is the email body (SMTP).\r\n")
	})
}

func (su *SMTPSystemTestSuite) TestSmokeSMTPForwardAcceptsEMailWithAttachments() {
	port := DynamicPort()
	proxyEndpoint := fmt.Sprintf("%s:%d", BindHost, port)
	smtpHost, err := su.smtpd.Host(su.ctx)
	require.NoError(su.T(), err)
	_smtpPort, err := su.smtpd.MappedPort(su.ctx, "5025/tcp")
	require.NoError(su.T(), err)
	smtpPort := strings.SplitN(string(_smtpPort), "/", 2)[0]
	apiEndpoint, err := su.smtpd.PortEndpoint(su.ctx, "5080/tcp", "http")
	require.NoError(su.T(), err)
	config := fmt.Sprintf(`
smtpd-proxy:
  listen: %s
  ehlo: localhost
  username: user@example.com
  password: password
  is_anon_auth_allowed: false
  upstream-servers:
  - type: smtp
    settings:
      addr: %s:%s
      host: %s
      auth: anon
`, proxyEndpoint, smtpHost, smtpPort, smtpHost)
	RunMainWithConfig(su.T(), config, port, func(t *testing.T, conn net.Conn) {
		// Setup authentication information.
		auth := smtp.PlainAuth("", "user@example.com", "password", BindHost)
		envelope := email.NewEmail()
		envelope.To = []string{"<discard-attachment@tld.invalid>"}
		envelope.From = "<gotest-attachment@esmtp.email>"
		envelope.Subject = "Subject: Test E-mail!"
		envelope.Text = []byte("This is the email body (SMTP).")
		envelope.Sender = "recipient@example.net"
		_, err := envelope.AttachFile("_testData/text-attachment.txt")
		require.NoError(t, err, "failed to attach file")
		err = envelope.Send(proxyEndpoint, auth)
		require.NoError(t, err, "failed to send message")

		fakerEmailReceived := requireFakerReceivedEmailWithContains(t, apiEndpoint, envelope.From)
		assert.Contains(t, fakerEmailReceived.RawData, "\r\nThis is the email body (SMTP).\r\n")
		assert.Contains(t, fakerEmailReceived.RawData, convertToBase64Fragment("Lorem ipsum dolor sit amet, consectetur adipiscing elit."))
	})
}

func convertToBase64Fragment(s string) string {
	out := base64.StdEncoding.EncodeToString([]byte(s))
	for strings.HasSuffix(out, "=") {
		out = strings.TrimSuffix(out, "=")
	}
	return out
}

func initFakeSMTPContainer(ctx context.Context) (container tc.Container, err error) {
	localstackReq := tc.ContainerRequest{
		Image:        "gessnerfl/fake-smtp-server",
		ExposedPorts: []string{"5080/tcp", "5081/tcp", "5025/tcp"},
		Env:          map[string]string{},
		WaitingFor:   wait.ForListeningPort("5080/tcp"),
	}

	container, err = tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: localstackReq,
		Started:          true,
	})
	return
}

// GET http://127.0.0.1:52472/api/email?page=0&size=10&sort=DESC
//
//	[{
//	    "id": 0,
//	    "fromAddress": "string",
//	    "toAddress": "string",
//	    "subject": "string",
//	    "receivedOn": "2022-09-01T19:30:05.509Z",
//	    "rawData": "string",
//	    "attachments": [
//	      {
//	        "id": 0,
//	        "filename": "string",
//	        "data":  "string"
//	      }
//	    ]
//	  }]
//
// via https://transform.tools/json-to-go
type fakerAPIEmail struct {
	ID          int       `json:"id"`
	FromAddress string    `json:"fromAddress"`
	ToAddress   string    `json:"toAddress"`
	Subject     string    `json:"subject"`
	ReceivedOn  time.Time `json:"receivedOn"`
	RawData     string    `json:"rawData"`
	Attachments []struct {
		ID       int    `json:"id"`
		Filename string `json:"filename"`
		Data     string `json:"data"`
	} `json:"attachments"`
}

func requireFakerReceivedEmailWithContains(t *testing.T, fakerAPIBaseURL, needle string) *fakerAPIEmail {
	var forwardedEmail *fakerAPIEmail

	assert.Eventuallyf(t,
		func() bool {
			// GET http://127.0.0.1:52472/api/email?page=0&size=10&sort=DESC
			params := url.Values{}
			params.Add("page", "0")
			params.Add("size", "255")
			params.Add("sort", "DESC")
			res, err := http.Get(fmt.Sprintf("%s/api/email?%s", fakerAPIBaseURL, params.Encode()))
			require.NoError(t, err)
			defer res.Body.Close()

			var data []fakerAPIEmail
			if body, err := io.ReadAll(res.Body); err == nil {
				bodyAsString := string(body)
				if !strings.Contains(bodyAsString, needle) {
					return false
				}
				err = json.Unmarshal(body, &data)
				require.NoError(t, err)
			} else {
				return false
			}

			for _, entry := range data {
				if strings.Contains(entry.RawData, needle) {
					scopedReference := entry
					forwardedEmail = &scopedReference
					break
				}
			}
			return forwardedEmail != nil
		},
		time.Second,
		10*time.Millisecond,
		"Failed to obtain ses payloads for %s", needle,
	)
	require.NotNil(t, forwardedEmail)
	return forwardedEmail
}
