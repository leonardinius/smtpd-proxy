package server

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/emersion/go-smtp"
	"github.com/leonardinius/smtpd-proxy/app/upstream"
)

// ErrorAuthCredentials error for invalid authentication
var ErrorAuthCredentials = errors.New("invalid username or password")

// ErrorAuthAnonCredentials error for unauthorized access
var ErrorAuthAnonCredentials = errors.New("user has not authenticated. anonymous access is not allowed")

// The backend implements SMTP server methods.
type backend struct {
	logger        *slog.Logger
	authLoginFunc AuthFunc
	isAnonAllowed bool
	forwarder     upstream.Registry
	ctx           context.Context
}

// The session implements SMTP session methods.
type session struct {
	bkd        *backend
	authorized bool
}

// NewBackend Creates new backend
func newBackend(ctx context.Context, logger *slog.Logger, authLoginFunc AuthFunc) *backend {
	return &backend{logger: logger, authLoginFunc: authLoginFunc, ctx: ctx}
}

var _ smtp.Backend = (*backend)(nil)

func (bkd *backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &session{bkd: bkd}, nil
}

var _ smtp.Session = (*session)(nil)

// Check if user is authorized or anon login is allowed
func (s *session) isAuthOk() error {
	if s.bkd.isAnonAllowed || s.authorized {
		return nil
	}

	return ErrorAuthAnonCredentials
}

func (s *session) AuthPlain(username, password string) error {
	err := s.bkd.authLoginFunc.Authenticate(username, password)
	s.authorized = err == nil
	s.bkd.logger.DebugContext(s.bkd.ctx, "auth plain", "username", username, "authorized", s.authorized)
	return err
}

// Set return path for currently processed message.
func (s *session) Mail(from string, opts *smtp.MailOptions) error {
	err := s.isAuthOk()
	s.bkd.logger.DebugContext(s.bkd.ctx, "mail", "from", from, "err", err)
	return err
}

// Add recipient for currently processed message.
func (s *session) Rcpt(to string, opts *smtp.RcptOptions) error {
	err := s.isAuthOk()
	s.bkd.logger.DebugContext(s.bkd.ctx, "rcpt", "to", to, "err", err)
	return err
}

// Set currently processed message contents and send it.
func (s *session) Data(r io.Reader) (err error) {
	if err = s.isAuthOk(); err != nil {
		return
	}

	var envelope *upstream.Email
	if envelope, err = upstream.NewEmailFromReader(r); err != nil {
		s.bkd.logger.ErrorContext(s.bkd.ctx, "data", "err", err)
		return err
	} else {
		s.bkd.logger.DebugContext(s.bkd.ctx, "data", "err", nil)
	}

	return s.bkd.forwarder.Forward(s.bkd.ctx, envelope)
}

// Discard currently processed message.
func (s *session) Reset() {
	s.bkd.logger.DebugContext(s.bkd.ctx, "reset")
}

// Free all resources associated with session.
func (s *session) Logout() error {
	s.bkd.logger.DebugContext(s.bkd.ctx, "logout")
	s.authorized = false
	return nil
}

// AuthFunc authentitate function type
type AuthFunc interface {
	Authenticate(username, password string) error
}

type authFunc func(username, password string) error

var _ AuthFunc = (*authFunc)(nil)

func (f authFunc) Authenticate(username, password string) error {
	return f(username, password)
}

// NoOpAuthFunc default auth forbidden auth function
func NoOpAuthFunc() AuthFunc {
	return authFunc(func(username, password string) error {
		return ErrorAuthCredentials
	})
}

// NewHardcodedAuthFunc hardcoded credentials auth function
func NewHardcodedAuthFunc(username, password string) AuthFunc {
	return authFunc(func(_username, _password string) error {
		if username != _username || password != _password {
			return ErrorAuthCredentials
		}
		return nil
	})
}
