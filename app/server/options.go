package server

import (
	"crypto/tls"

	"github.com/leonardinius/smtpd-proxy/app/upstream"
)

// An Option configures a server.
type Option interface {
	apply(*SrvBackend)
}

// optionFunc wraps a func so it satisfies the Option interface.
type optionFunc func(*SrvBackend)

func (f optionFunc) apply(srv *SrvBackend) {
	f(srv)
}

// WithAuth sets auth
func WithAuth(auth AuthFunc) Option {
	return optionFunc(func(srv *SrvBackend) {
		srv.backend.authLoginFunc = auth
	})
}

// WithAnnonAuthAllowed whether to allow anon login or not, added for perf tests
func WithAnnonAuthAllowed(isAnonAllowed bool) Option {
	return optionFunc(func(srv *SrvBackend) {
		srv.backend.isAnonAllowed = isAnonAllowed
	})
}

// WithTLSConfig sets tls
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return optionFunc(func(srv *SrvBackend) {
		srv.smtp.TLSConfig = tlsConfig
	})
}

// WithUpstreamServers sets the upstream server handler
func WithUpstreamServers(reg upstream.Registry) Option {
	return optionFunc(func(srv *SrvBackend) {
		srv.backend.forwarder = reg
	})
}
