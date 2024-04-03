package server

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/leonardinius/smtpd-proxy/app/upstream"
)

var (
	// ErrInvalidIdentity error for invalid identity (domain check).
	ErrInvalidIdentity = errors.New("invalid identity")

	// ErrAuthCredentials error for invalid authentication.
	ErrAuthCredentials = errors.New("invalid username or password")

	// ErrAuthAnonCredentials error for unauthorized access.
	ErrAuthAnonCredentials = errors.New("user has not authenticated. anonymous access is not allowed")

	// ErrUnsupportedMechanism error for unsupported mechanism.
	ErrUnsupportedMechanism = errors.New("unsupported authentication mechanism")
)

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
	conn       *smtp.Conn
	authorized bool
}

// NewBackend Creates new backend.
func newBackend(ctx context.Context, logger *slog.Logger, authLoginFunc AuthFunc) *backend {
	return &backend{logger: logger, authLoginFunc: authLoginFunc, ctx: ctx}
}

var _ smtp.Backend = (*backend)(nil)

func (bkd *backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &session{bkd: bkd, conn: c}, nil
}

var _ smtp.AuthSession = (*session)(nil)

// Check if user is authorized or anon login is allowed.
func (s *session) isAuthOk() error {
	if s.bkd.isAnonAllowed || s.authorized {
		return nil
	}

	return ErrAuthAnonCredentials
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

func (s *session) AuthMechanisms() []string {
	return []string{sasl.Plain, sasl.Login}
}

func (s *session) Auth(mech string) (sasl.Server, error) {
	switch mech {
	case sasl.Plain:
		return sasl.NewPlainServer(func(identity, username, password string) error {
			err := s.bkd.authLoginFunc.Authenticate(identity, username, password)
			s.authorized = err == nil
			s.bkd.logger.DebugContext(s.bkd.ctx, "auth plain", "username", username, "authorized", s.authorized)
			return err
		}), nil
	case sasl.Login:
		return sasl.NewLoginServer(func(username, password string) error {
			err := s.bkd.authLoginFunc.Authenticate("", username, password)
			s.authorized = err == nil
			s.bkd.logger.DebugContext(s.bkd.ctx, "auth login", "username", username, "authorized", s.authorized)
			return err
		}), nil
	default:
		return nil, ErrUnsupportedMechanism
	}
}

// AuthFunc authentitate function type.
type AuthFunc interface {
	Authenticate(identity, username, password string) error
}

type authFunc func(identity, username, password string) error

var _ AuthFunc = (*authFunc)(nil)

func (f authFunc) Authenticate(identity, username, password string) error {
	return f(identity, username, password)
}

// NoOpAuthFunc default auth forbidden auth function.
func NoOpAuthFunc() AuthFunc {
	return authFunc(func(identity, username, password string) error {
		return ErrAuthCredentials
	})
}

// NewHardcodedAuthFunc hardcoded credentials auth function.
func NewHardcodedAuthFunc(identity, username, password string) AuthFunc {
	return authFunc(func(_identity, _username, _password string) error {
		if _identity != "" && identity != _identity {
			return ErrInvalidIdentity
		}

		if username != _username || password != _password {
			return ErrAuthCredentials
		}
		return nil
	})
}
