package forwarder

import (
	"context"
	"encoding/json"
	gohttp "net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/leonardinius/smtpd-proxy/app/upstream"
)

const charSet = "UTF-8"
const maxConnections = 100

// sesUpstreamSettings AWS SES upstream details
type sesUpstreamSettings struct {
	AwsAccessKeyID     string `json:"aws_access_key_id"`
	AwsSecretAccessKey string `json:"aws_secret_access_key"`
	Region             string `json:"region"`
	Endpoint           string `json:"endpoint"`
}

type sesUpstream struct {
	settings sesUpstreamSettings
	ses      *ses.Client
}

var _ upstream.Server = (*sesUpstream)(nil)
var _ upstream.Forwarder = (*sesUpstream)(nil)

// NewSESServer new ses upstream
func NewSESServer() upstream.Server {
	return new(sesUpstream)
}

func (u *sesUpstream) Configure(ctx context.Context, settings map[string]any) (upstream.Forwarder, error) {
	bytes, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &u.settings)
	if err != nil {
		return nil, err
	}

	c := &u.settings
	credentialsProvider := credentials.NewStaticCredentialsProvider(c.AwsAccessKeyID, c.AwsSecretAccessKey, "")
	httpClient := http.NewBuildableClient().WithTransportOptions(func(tr *gohttp.Transport) {
		tr.MaxIdleConns = maxConnections
		tr.MaxIdleConnsPerHost = maxConnections
		tr.MaxConnsPerHost = 0
	})
	endpointResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if c.Region != "" {
			region = c.Region
		}
		if c.Endpoint != "" {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           c.Endpoint,
				SigningRegion: region,
			}, nil
		}

		// returning EndpointNotFoundError will allow the service to fallback to its default resolution
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithEndpointResolverWithOptions(endpointResolver),
		config.WithRegion(c.Region),
		config.WithCredentialsProvider(credentialsProvider),
		config.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, err
	}

	u.ses = ses.NewFromConfig(cfg)
	return u, nil
}

func (u *sesUpstream) Forward(ctx context.Context, mail *upstream.Email) (err error) {
	if len(mail.Attachments) == 0 {
		return u.sesForwardSimple(ctx, mail)
	}

	return u.sesForwardRaw(ctx, mail)
}

func (u *sesUpstream) sesForwardRaw(ctx context.Context, mail *upstream.Email) error {
	bytes, err := mail.Bytes()
	if err != nil {
		return err
	}

	destinations := make([]string, 0, 3)
	destinations = append(destinations, mail.To...)
	destinations = append(destinations, mail.Bcc...)
	destinations = append(destinations, mail.Cc...)

	inputRaw := &ses.SendRawEmailInput{
		Source:       aws.String(mail.From),
		Destinations: destinations,
		RawMessage:   &types.RawMessage{Data: bytes},
	}

	// Attempt to send the email.
	_, err = u.ses.SendRawEmail(ctx, inputRaw)
	return err
}

func (u *sesUpstream) sesForwardSimple(ctx context.Context, mail *upstream.Email) error {
	input := &ses.SendEmailInput{
		Source: aws.String(mail.From),
		Destination: &types.Destination{
			ToAddresses:  mail.To,
			BccAddresses: mail.Bcc,
			CcAddresses:  mail.Cc,
		},
		ReplyToAddresses: mail.ReplyTo,
		Message: &types.Message{
			Body: &types.Body{
				Html: &types.Content{
					Charset: aws.String(charSet),
					Data:    aws.String(string(mail.HTML)),
				},
				Text: &types.Content{
					Charset: aws.String(charSet),
					Data:    aws.String(string(mail.Text)),
				},
			},
			Subject: &types.Content{
				Charset: aws.String(charSet),
				Data:    aws.String(mail.Subject),
			},
		},
	}

	// Attempt to send the email.
	_, err := u.ses.SendEmail(ctx, input)
	return err
}
