package systemtest

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	awsses "github.com/aws/aws-sdk-go-v2/service/ses"
	endpoints "github.com/aws/smithy-go/endpoints"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	_ suite.SetupAllSuite    = (*SESSystemTestSuite)(nil)
	_ suite.TearDownAllSuite = (*SESSystemTestSuite)(nil)
)

// SESSystemTestSuite suite.
type SESSystemTestSuite struct {
	suite.Suite
	ctx        context.Context
	localstack tc.Container
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run.
func TestSESSystemTestSuite(t *testing.T) {
	suite.Run(t, new(SESSystemTestSuite))
}

func (su *SESSystemTestSuite) SetupSuite() {
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	su.T().Cleanup(cancel)
	su.ctx = ctx

	su.localstack, err = iniFakeSesSMTPContainer(su.ctx)
	if err != nil {
		su.T().Fatalf("Errors: %v ", err)
	}

	su.T().Parallel()
}

func (su *SESSystemTestSuite) /*  */ TearDownSuite() {
	container := su.localstack
	tc.CleanupContainer(su.T(), container)
	su.localstack = nil
}

func (su *SESSystemTestSuite) TestSmokeSESForwardAcceptsSimpleEMail() {
	port := DynamicPort()
	proxyEndpoint := fmt.Sprintf("%s:%d", BindHost, port)
	sesPort, err := su.localstack.MappedPort(su.ctx, "4566/tcp")
	require.NoError(su.T(), err)
	sesEndpoint := "http://" + net.JoinHostPort(BindHost, sesPort.Port())
	config := fmt.Sprintf(`
smtpd-proxy:
  listen: %s:%d
  ehlo: 127.0.0.1
  username: user-ses@example.com
  password: password
  is_anon_auth_allowed: false
  upstream-servers:
  - type: ses
    settings:
      endpoint: %s
      aws_access_key_id: amz-key-1
      aws_secret_access_key: amz-**-secret
      region: us-east-1
`, BindHost, port, sesEndpoint)
	RunMainWithConfig(su.ctx, su.T(), config, port, func(t *testing.T, conn net.Conn) {
		fromEmail := fmt.Sprintf("<gotest-%d-simple@esmtp.email>", time.Now().UnixMilli())
		// Setup authentication information.
		auth := smtp.PlainAuth("", "user-ses@example.com", "password", BindHost)
		to := []string{"recipient-ses@example.net"}
		msg := strings.Join([]string{
			"To: <discard-simple-ses@tld.invalid>",
			"From: " + fromEmail,
			"Subject: Test E-mail! (SES)",
			"",
			"This is the email body (SES).",
			"",
		}, "\r\n")
		err := smtp.SendMail(proxyEndpoint, auth, "sender-ses@example.org", to, []byte(msg))
		require.ErrorContains(t, err, "Email address not verified")

		ses := newSesClient(su.ctx, t, sesEndpoint)
		_, err = ses.VerifyEmailIdentity(su.ctx, &awsses.VerifyEmailIdentityInput{EmailAddress: aws.String(fromEmail)})
		require.NoError(t, err)
		err = smtp.SendMail(proxyEndpoint, auth, "sender-ses@example.org", to, []byte(msg))
		assert.NoError(t, err)

		sesFile := requireSesFileWithContains(t, fromEmail)
		bytes, err := io.ReadAll(sesFile)
		require.NoError(t, err)
		jsonMessage := string(bytes)
		assert.Contains(t, jsonMessage,
			"\"This is the email body (SES).\\r\\n\"")
	})
}

func (su *SESSystemTestSuite) TestSmokeSESForwardAcceptsEMailWithAttachments() {
	port := DynamicPort()
	proxyEndpoint := fmt.Sprintf("%s:%d", BindHost, port)
	sesPort, err := su.localstack.MappedPort(su.ctx, "4566/tcp")
	require.NoError(su.T(), err)
	sesEndpoint := "http://" + net.JoinHostPort(BindHost, sesPort.Port())
	config := fmt.Sprintf(`
smtpd-proxy:
  listen: %s:%d
  ehlo: 127.0.0.1
  username: user-ses@example.com
  password: password
  is_anon_auth_allowed: false
  upstream-servers:
  - type: ses
    settings:
      endpoint: %s
      aws_access_key_id: amz-key-1
      aws_secret_access_key: amz-**-secret
      region: us-east-1
`, BindHost, port, sesEndpoint)
	RunMainWithConfig(su.ctx, su.T(), config, port, func(t *testing.T, conn net.Conn) {
		fromEmail := fmt.Sprintf("<gotest-%d-attachment@esmtp.email>", time.Now().UnixMilli())
		ses := newSesClient(su.ctx, t, sesEndpoint)
		_, err = ses.VerifyEmailIdentity(su.ctx, &awsses.VerifyEmailIdentityInput{EmailAddress: aws.String(fromEmail)})
		require.NoError(t, err, "failed to verify attachment email")

		// Setup authentication information.
		auth := smtp.PlainAuth("", "user-ses@example.com", "password", BindHost)
		envelope := email.NewEmail()
		envelope.To = []string{"<discard-attachment-ses@tld.invalid>"}
		envelope.From = fromEmail
		envelope.Subject = "Subject: Test Ses E-mail!"
		envelope.Text = []byte("This is the email body (SES).")
		envelope.Sender = "recipient-ses@example.net"
		_, err := envelope.AttachFile("_testData/text-attachment.txt")
		require.NoError(t, err, "failed to attach file")
		err = envelope.Send(proxyEndpoint, auth)
		assert.NoError(t, err, "failed to resend e-mail")

		// assert that the file was sent
		sesFile := requireSesFileWithContains(t, envelope.From)
		bytes, err := io.ReadAll(sesFile)
		require.NoError(t, err)
		jsonMessage := string(bytes)
		loremIpsumBase64 := base64.StdEncoding.EncodeToString([]byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit."))
		for strings.HasSuffix(loremIpsumBase64, "=") {
			loremIpsumBase64 = strings.TrimSuffix(loremIpsumBase64, "=")
		}
		assert.Contains(t, jsonMessage, "\\r\\nThis is the email body (SES).\\r\\n")
		assert.Contains(t, jsonMessage, loremIpsumBase64)
	})
}

func iniFakeSesSMTPContainer(ctx context.Context) (tc.Container, error) {
	vol, _ := filepath.Abs(".volume")
	_ = os.Mkdir(vol, 0o755)
	_ = os.RemoveAll(vol + "/state/ses")
	localstackReq := tc.ContainerRequest{
		Image:        "localstack/localstack:2.3.2",
		ExposedPorts: []string{"4566/tcp"},
		Env: map[string]string{
			"EAGER_SERVICE_LOADING": "1",
			"SERVICES":              "ses",
			"DEBUG":                 "1",
			"PERSISTENCE":           "1",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.AutoRemove = true
			hc.Mounts = []mount.Mount{
				{
					Source: vol,
					Target: "/var/lib/localstack",
					Type:   mount.TypeBind,
				},
			}
		},
		WaitingFor: wait.ForListeningPort("4566/tcp"),
	}
	instance, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: localstackReq,
		Started:          true,
	})
	return instance, err
}

func newSesClient(ctx context.Context, t *testing.T, endpoint string) *awsses.Client {
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	v2Resolver := &v2EndpointResolver{Endpoint: u, Headers: http.Header{}}

	credentialsProvider := awscreds.NewStaticCredentialsProvider("amz-key-1", "amz-**-secret", "")
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentialsProvider),
	)
	require.NoError(t, err)
	return awsses.NewFromConfig(cfg, awsses.WithEndpointResolverV2(v2Resolver))
}

func requireSesFileWithContains(t *testing.T, needle string) *os.File {
	var sesFile *os.File
	const mailSesJSONDir = ".volume/state/ses/"
	assert.Eventuallyf(t,
		func() bool {
			dir, e := filepath.Abs(mailSesJSONDir)
			if e != nil {
				return false
			}
			entries, e := os.ReadDir(dir)
			if e != nil {
				return false
			}

			for _, entry := range entries {
				file, e := os.Open(filepath.Join(dir, entry.Name()))
				if e != nil {
					continue
				}
				bytes, e := io.ReadAll(file)
				if e != nil {
					continue
				}
				if strings.Contains(string(bytes), needle) {
					sesFile = file
					break
				}
			}
			return sesFile != nil
		},
		10*time.Second,
		300*time.Millisecond,
		"Failed to obtain ses payloads for %s", needle,
	)
	require.NotNil(t, sesFile)
	_, err := sesFile.Seek(0, 0)
	require.NoError(t, err)
	return sesFile
}

type v2EndpointResolver struct {
	Endpoint *url.URL
	Headers  http.Header
}

// ResolveEndpoint implements ses.EndpointResolverV2.
func (r *v2EndpointResolver) ResolveEndpoint(ctx context.Context, params awsses.EndpointParameters) (endpoints.Endpoint, error) {
	return endpoints.Endpoint{
		URI:     *r.Endpoint,
		Headers: r.Headers,
	}, nil
}

var _ awsses.EndpointResolverV2 = (*v2EndpointResolver)(nil)
