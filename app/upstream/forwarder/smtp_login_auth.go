package forwarder

// MIT license (c) andelf 2013
// https://gist.githubusercontent.com/homme/22b457eb054a07e7b2fb/raw/baa7af8820810be2dd559ae9f476f8e9df94770f/smtp_login_auth.go

import (
	"errors"
	"net/smtp"
)

var (
	errUnecryptedConnection      = errors.New("unencrypted connection")
	errWrongHostName             = errors.New("wrong host name")
	errUnknownResponseFromServer = errors.New("unknown response from server")
)

var _ smtp.Auth = (*loginAuth)(nil)

type loginAuth struct {
	username, password, host string
}

// NewLoginAuth return smtp auth LOGIN authentication.
func NewLoginAuth(username, password, host string) smtp.Auth {
	return &loginAuth{username, password, host}
}

// Start smtp.Auth interface.
func (a *loginAuth) Start(server *smtp.ServerInfo) (proto string, toServer []byte, err error) {
	if err := a.checkCopiedFromStdPlainAuth(server); err != nil {
		return "LOGIN", nil, err
	}

	return "LOGIN", []byte(a.username), nil
}

func (a *loginAuth) checkCopiedFromStdPlainAuth(server *smtp.ServerInfo) error {
	// Must have TLS, or else localhost server.
	// Note: If TLS is not true, then we can't trust ANYTHING in ServerInfo.
	// In particular, it doesn't matter if the server advertises PLAIN auth.
	// That might just be the attacker saying
	// "it's ok, you can trust me with your password."
	if !server.TLS && !isLocalhost(server.Name) {
		return errUnecryptedConnection
	}
	if server.Name != a.host {
		return errWrongHostName
	}
	return nil
}

// Next smtp.Auth interface.
func (a *loginAuth) Next(fromServer []byte, more bool) (toServer []byte, err error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errUnknownResponseFromServer
		}
	}
	return nil, nil
}

func isLocalhost(name string) bool {
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}

// usage:
// auth := LoginAuth("loginname", "password")
// err := smtp.SendMail(smtpServer + ":25", auth, fromAddress, toAddresses, []byte(message))
// or
// client, err := smtp.Dial(smtpServer)
// client.Auth(LoginAuth("loginname", "password"))
