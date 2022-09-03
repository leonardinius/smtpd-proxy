package forwarder

import (
	"context"
	"encoding/json"

	"github.com/leonardinius/smtpd-proxy/app/upstream"
	"github.com/leonardinius/smtpd-proxy/app/zlog"
)

type logUpstreamSettings struct {
}

type logServer struct {
	settings logUpstreamSettings
}

var _ upstream.Server = (*logServer)(nil)
var _ upstream.Forwarder = (*logServer)(nil)

// NewLogServer new ses upstream
func NewLogServer() upstream.Server {
	return new(logServer)
}

func (u *logServer) Configure(settings map[string]any) (upstream.Forwarder, error) {
	bytes, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &u.settings)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func (u *logServer) Forward(ctx context.Context, mail *upstream.Email) error {
	var uid string
	if meta, ok := upstream.FromContext(ctx); ok {
		uid = meta.UID
	}
	text := string(([]rune(string(mail.Text)))[:20]) + "..."

	zlog.Infow("log-forwarder",
		"uid", uid,
		"From", mail.From,
		"To", mail.To,
		"ReplyTo", mail.ReplyTo,
		"Cc", mail.Cc,
		"Bcc", mail.Bcc,
		"Subject", mail.Subject,
		"Excerpt", text,
	)

	return nil
}
