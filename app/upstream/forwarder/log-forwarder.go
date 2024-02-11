package forwarder

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/leonardinius/smtpd-proxy/app/upstream"
)

type logUpstreamSettings struct {
}

type logServer struct {
	settings logUpstreamSettings
	logger   *slog.Logger
}

var _ upstream.Server = (*logServer)(nil)
var _ upstream.Forwarder = (*logServer)(nil)

// NewLogServer new ses upstream
func NewLogServer(logger *slog.Logger) upstream.Server {
	return &logServer{logger: logger}
}

func (u *logServer) Configure(ctx context.Context, settings map[string]any) (upstream.Forwarder, error) {
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

	u.logger.InfoContext(ctx, "log-forwarder",
		"uid", uid,
		"from", mail.From,
		"to", mail.To,
		"replyTo", mail.ReplyTo,
		"cc", mail.Cc,
		"bcc", mail.Bcc,
		"subject", mail.Subject,
		"excerpt", text,
	)

	return nil
}
