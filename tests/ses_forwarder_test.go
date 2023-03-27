package systemtest

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	awsses "github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var _ suite.SetupAllSuite = (*SESSystemTestSuite)(nil)
var _ suite.TearDownAllSuite = (*SESSystemTestSuite)(nil)

// SESSystemTestSuite suite
type SESSystemTestSuite struct {
	suite.Suite
	ctx        context.Context
	localstack tc.Container
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSESSystemTestSuite(t *testing.T) {
	suite.Run(t, new(SESSystemTestSuite))
}

func (su *SESSystemTestSuite) SetupSuite() {
	var err error
	su.ctx = context.Background()
	su.localstack, err = iniFakeSMTPContainer(su.ctx)
	if err != nil {
		su.T().Fatalf("Errors: %v ", err)
	}
}

func (su *SESSystemTestSuite) /*  */ TearDownSuite() {
	err := TerminateContainer(su.ctx, su.localstack)
	require.NoError(su.T(), err)
	su.localstack = nil
}

func (su *SESSystemTestSuite) TestSmokeSESForwardAcceptsSimpleEMail() {
	port := DynamicPort()
	proxyEndpoint := fmt.Sprintf("%s:%d", BindHost, port)
	sesEndpoint, err := su.localstack.PortEndpoint(su.ctx, "4566/tcp", "http")
	require.NoError(su.T(), err)
	config := fmt.Sprintf(`
smtpd-proxy:
  listen: %s:%d
  ehlo: localhost
  username: user@example.com
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
	RunMainWithConfig(su.T(), config, port, func(t *testing.T, conn net.Conn) {
		fromEmail := "<gotest-simple@esmtp.email>"
		// Setup authentication information.
		auth := smtp.PlainAuth("", "user@example.com", "password", BindHost)
		to := []string{"recipient@example.net"}
		msg := strings.Join([]string{
			"To: <discard-simple@tld.invalid>",
			"From: " + fromEmail,
			"Subject: Test E-mail!",
			"",
			"This is the email body.",
			"",
		}, "\r\n")
		err := smtp.SendMail(proxyEndpoint, auth, "sender@example.org", to, []byte(msg))
		require.ErrorContains(t, err, "Email address not verified <gotest-simple@esmtp.email>")

		ses := newSesClient(t, sesEndpoint)
		_, err = ses.VerifyEmailIdentity(su.ctx, &awsses.VerifyEmailIdentityInput{EmailAddress: aws.String(fromEmail)})
		require.NoError(t, err)
		err = smtp.SendMail(proxyEndpoint, auth, "sender@example.org", to, []byte(msg))
		assert.NoError(t, err)

		sesFile := requireSesFileWithContains(t, fromEmail)
		bytes, err := io.ReadAll(sesFile)
		require.NoError(t, err)
		jsonMessage := string(bytes)
		assert.Contains(t, jsonMessage,
			"\"This is the email body.\\r\\n\"")
	})
}

func (su *SESSystemTestSuite) TestSmokeSESForwardAcceptsEMailWithAttachments() {
	port := DynamicPort()
	proxyEndpoint := fmt.Sprintf("%s:%d", BindHost, port)
	sesEndpoint, err := su.localstack.PortEndpoint(su.ctx, "4566/tcp", "http")
	require.NoError(su.T(), err)
	config := fmt.Sprintf(`
smtpd-proxy:
  listen: %s:%d
  ehlo: localhost
  username: user@example.com
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
	RunMainWithConfig(su.T(), config, port, func(t *testing.T, conn net.Conn) {
		ses := newSesClient(t, sesEndpoint)
		_, err = ses.VerifyEmailIdentity(su.ctx, &awsses.VerifyEmailIdentityInput{EmailAddress: aws.String("<gotest-attachment@esmtp.email>")})
		require.NoError(t, err, "failed to verify gotest-attachment@esmtp.email")

		// Setup authentication information.
		auth := smtp.PlainAuth("", "user@example.com", "password", BindHost)
		envelope := email.NewEmail()
		envelope.To = []string{"<discard-attachment@tld.invalid>"}
		envelope.From = "<gotest-attachment@esmtp.email>"
		envelope.Subject = "Subject: Test E-mail!"
		envelope.Text = []byte("This is the email body.")
		envelope.Sender = "recipient@example.net"
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
		assert.Contains(t, jsonMessage, "\\r\\nThis is the email body.\\r\\n")
		assert.Contains(t, jsonMessage, loremIpsumBase64)
	})
}

func iniFakeSMTPContainer(ctx context.Context) (container tc.Container, err error) {
	vol, _ := filepath.Abs(".volume")
	_ = os.Mkdir(vol, 0o755)
	_ = os.RemoveAll(filepath.Join(vol, "tmp", "state", "ses"))
	localstackReq := tc.ContainerRequest{
		Image:        "localstack/localstack",
		ExposedPorts: []string{"4566/tcp"},
		Env: map[string]string{
			"EAGER_SERVICE_LOADING": "1",
			"SERVICES":              "ses",
		},
		Mounts:     tc.Mounts(tc.BindMount(vol, "/var/lib/localstack")),
		WaitingFor: wait.ForListeningPort("4566/tcp"),
	}

	container, err = tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: localstackReq,
		Started:          true,
	})
	return
}

func newSesClient(t *testing.T, endpoint string) *awsses.Client {
	endpointResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:   "aws",
			URL:           endpoint,
			SigningRegion: "us-east-1",
		}, nil
	})
	credentialsProvider := awscreds.
		NewStaticCredentialsProvider("amz-key-1", "amz-**-secret", "")
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithEndpointResolverWithOptions(endpointResolver),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentialsProvider),
	)
	require.NoError(t, err)
	return awsses.NewFromConfig(cfg)
}

func requireSesFileWithContains(t *testing.T, needle string) *os.File {
	var sesFile *os.File
	const mailSesJSONDir = ".volume/tmp/state/ses/"
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
		5*time.Second,
		50*time.Millisecond,
		"Failed to obtain ses payloads for %s", needle,
	)
	require.NotNil(t, sesFile)
	_, err := sesFile.Seek(0, 0)
	require.NoError(t, err)
	return sesFile
}
