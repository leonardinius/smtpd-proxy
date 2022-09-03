package forwarder

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"

	"github.com/leonardinius/smtpd-proxy/app/upstream"
)

// smtpUpstreamSettings smtp details
type smtpUpstreamSettings struct {
	Addr     string `json:"addr"`
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
}

type smptUpstream struct {
	settings smtpUpstreamSettings
	auth     smtp.Auth
}

var _ upstream.Server = (*smptUpstream)(nil)
var _ upstream.Forwarder = (*smptUpstream)(nil)

// NewSMTPServer new smtp upstream
func NewSMTPServer() upstream.Server {
	return new(smptUpstream)
}

func (u *smptUpstream) Configure(settings map[string]any) (upstream.Forwarder, error) {
	bytes, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &u.settings)
	if err != nil {
		return nil, err
	}

	if u.auth, err = u.initAuth(); err != nil {
		return nil, err
	}

	return u, nil
}

func (u *smptUpstream) initAuth() (auth smtp.Auth, err error) {
	c := &u.settings
	host := c.Host
	if host == "" {
		if host, _, err = net.SplitHostPort(c.Addr); err != nil {
			return nil, err
		}
	}

	switch authType := c.Auth; authType {
	case "plain":
		auth = smtp.PlainAuth("", c.Username, c.Password, host)
	case "login":
		auth = NewLoginAuth(c.Username, c.Password, host)
	case "cram-md5":
		auth = smtp.CRAMMD5Auth(c.Username, c.Password)
	case "anon":
		auth = nil
	default:
		err = fmt.Errorf("unrecognized auth type: %s, supported values [login, plain, cram-md5, anon]", authType)
	}
	return
}

func (u *smptUpstream) Forward(_ context.Context, mail *upstream.Email) error {
	return mail.Send(u.settings.Addr, u.auth)
}
