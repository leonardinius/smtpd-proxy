package server

import (
	"context"
	"errors"
	"io"

	"github.com/emersion/go-smtp"
	"github.com/leonardinius/smtpd-proxy/app/upstream"
	"github.com/leonardinius/smtpd-proxy/app/zlog"
)

// ErrorAuthCredentials error for invalid authentication
var ErrorAuthCredentials = errors.New("invalid username or password")

// The backend implements SMTP server methods.
type backend struct {
	authLoginFunc AuthFunc
	isAnonAllowed bool
	forwarder     upstream.Registry
	ctx           context.Context
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

// NewBackend Creates new backend
func newBackend(authLoginFunc AuthFunc) *backend {
	return &backend{authLoginFunc: authLoginFunc, ctx: context.Background()}
}

var _ smtp.Backend = (*backend)(nil)

// Login handles a login command with username and password.
func (bkd *backend) Login(bkdtate *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	if err := bkd.authLoginFunc.Authenticate(username, password); err != nil {
		return nil, err
	}

	return bkd, nil
}

// AnonymousLogin requires clients to authenticate using SMTP AUTH before sending emails
func (bkd *backend) AnonymousLogin(bkdtate *smtp.ConnectionState) (smtp.Session, error) {
	if bkd.isAnonAllowed {
		return bkd, nil
	}

	return nil, ErrorAuthCredentials
}

var _ smtp.Session = (*backend)(nil)

func (bkd *backend) Mail(from string, opts smtp.MailOptions) error {
	zlog.Debugf("Mail from: %s", from)
	return nil
}

func (bkd *backend) Rcpt(to string) error {
	zlog.Debugf("Rcpt to: %s", to)
	return nil
}

func (bkd *backend) Data(r io.Reader) (err error) {
	var envelope *upstream.Email
	zlog.Debug("DATA")
	if envelope, err = upstream.NewEmailFromReader(r); err != nil {
		zlog.Error("Data err", err)
		return err
	}

	return bkd.forwarder.Forward(bkd.ctx, envelope)
}

func (bkd *backend) Reset() {
	zlog.Debug("Reset")
}

func (bkd *backend) Logout() error {
	zlog.Debug("Logout")
	return nil
}
