package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

var (
	_timeout = time.Duration(3)
	// ReadTimeout default 2 secs
	ReadTimeout = _timeout * time.Second
	// WriteTimeout default 2 secs
	WriteTimeout = _timeout * time.Second
	_mb          = 1024 * 1024
	// MaxMessageBytes default 10 Mb
	MaxMessageBytes = 10 * _mb
	// MaxRecipients default 50
	MaxRecipients = 50
	// AllowInsecureAuth default true
	AllowInsecureAuth = true
	// EnableSMTPUTF8 default true
	EnableSMTPUTF8 = true
)

// SMTPServer abstration
type SMTPServer interface {
	Shutdown() error
	ListenAndServe() error
}

type SrvBackend struct {
	smtp    *smtp.Server
	backend *backend
}

var _ SMTPServer = (*SrvBackend)(nil)

func (srv *SrvBackend) Shutdown() error {
	return srv.smtp.Close()
}

func (srv *SrvBackend) ListenAndServe() error {
	if srv.smtp.TLSConfig != nil {
		return srv.smtp.ListenAndServeTLS()
	}
	return srv.smtp.ListenAndServe()
}

// NewServer prepares SMTP server
func NewServer(ctx context.Context, logger *slog.Logger, addr, domain string) *SrvBackend {
	bkd := newBackend(ctx, logger, NoOpAuthFunc())
	s := smtp.NewServer(bkd)
	s.Addr = addr
	s.Domain = domain
	s.ReadTimeout = ReadTimeout
	s.WriteTimeout = WriteTimeout
	s.MaxMessageBytes = int64(MaxMessageBytes)
	s.MaxRecipients = MaxRecipients
	s.AllowInsecureAuth = AllowInsecureAuth
	s.EnableSMTPUTF8 = EnableSMTPUTF8

	s.EnableAuth(sasl.Login, func(conn *smtp.Conn) sasl.Server {
		return sasl.NewLoginServer(func(username, password string) error {
			return conn.Session().AuthPlain(username, password)
		})
	})

	return &SrvBackend{smtp: s, backend: bkd}
}

func (srv *SrvBackend) WithOptions(opts ...Option) *SrvBackend {
	for _, opt := range opts {
		opt.apply(srv)
	}
	return srv
}
