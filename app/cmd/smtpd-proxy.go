package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"errors"

	"github.com/hashicorp/go-multierror"
	"github.com/jessevdk/go-flags"
	"github.com/leonardinius/smtpd-proxy/app/config"
	"github.com/leonardinius/smtpd-proxy/app/server"
	"github.com/leonardinius/smtpd-proxy/app/upstream"
	"github.com/leonardinius/smtpd-proxy/app/upstream/forwarder"
)

var (
	// COMMIT git commit
	COMMIT = "gitsha1"
	// BRANCH git branch
	BRANCH = "dirty"
)

// Opts with all cli commands and flags
type Opts struct {
	ConfigYamlFile string `long:"configuration" short:"c" env:"SMTPD_CONFIG" required:"true" default:"smtpd-proxy.yml" description:"smtpd-proxy.yml configuration path"`
	Verbose        bool   `long:"verbose" short:"v" env:"VERBOSE" description:"verbose mode"`
}

var errorEmptyRegistry = errors.New("empty sender registry")

// Main function
func Main(ctx context.Context, args ...string) error {
	var opts Opts
	p := flags.NewParser(&opts, flags.Default)

	if _, err := p.ParseArgs(args); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			fmt.Printf("smtpd-proxy revision %s-%s\n", BRANCH, COMMIT)
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	fmt.Printf("smtpd-proxy revision %s-%s\n", BRANCH, COMMIT)
	opts.ConfigYamlFile = filepath.Clean(opts.ConfigYamlFile)
	slog.InfoContext(ctx, "parsing yaml", "path", opts.ConfigYamlFile)
	cfg, err := config.ParseFile(opts.ConfigYamlFile)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse configuration", "path", opts.ConfigYamlFile, "err", err)
		return err
	}
	cfg, err = cfg.LoadDefaults()
	if err != nil {
		slog.ErrorContext(ctx, "failed to load configuration", "path", opts.ConfigYamlFile, "err", err)
		return err
	}

	return ListenProxyAndServe(ctx, cfg)
}

// ListenProxyAndServe run proxy cmd
func ListenProxyAndServe(ctx context.Context, c *config.Config) error {
	srvConfig := c.ServerConfig
	logger := slog.Default().With("server", srvConfig.Listen, "ehlo", srvConfig.Ehlo)
	tlsConfig, err := loadTLSConfig(srvConfig.ServerCertificatePath, srvConfig.ServerKeyPath)
	if err != nil {
		return err
	}

	upstreamServers, err := createUpstreamServers(ctx, srvConfig.UpstreamServers)
	if err != nil {
		return err
	}

	if srvConfig.Ehlo == "" {
		srvConfig.Ehlo, _, _ = net.SplitHostPort(srvConfig.Listen)
	}

	srv := server.NewServer(
		ctx,
		srvConfig.Listen,
		srvConfig.Ehlo,
	).WithOptions(
		server.WithAuth(server.NewHardcodedAuthFunc(srvConfig.Username, srvConfig.Password)),
		server.WithAnnonAuthAllowed(srvConfig.IsAnonAuthAllowed),
		server.WithTLSConfig(tlsConfig),
		server.WithUpstreamServers(upstreamServers),
	)

	errCh := make(chan error, 1)

	go func() {
		if x := recover(); x != nil {
			logger.WarnContext(ctx, "run time panic", "panic", x)
			panic(x)
		}

		logger.InfoContext(ctx, "starting server")
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.InfoContext(ctx, "shutting down")
		if err := srv.Shutdown(); err != nil {
			logger.ErrorContext(ctx, "failed to shutdown server", "err", err)
			return err
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func loadTLSConfig(serverCertificatePath, serverKeyPath string) (*tls.Config, error) {
	if serverCertificatePath == "" && serverKeyPath == "" {
		return nil, nil
	}
	cer, err := tls.LoadX509KeyPair(serverCertificatePath, serverKeyPath)
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cer}, MinVersion: tls.VersionTLS12}, nil
}

func createUpstreamServers(ctx context.Context, upstreamServersConfig []config.UpstreamServer) (reg upstream.Registry, err error) {
	reg = upstream.NewEmptyRegistry()
	for _, serverConfig := range upstreamServersConfig {
		var handler upstream.Forwarder
		var _err error

		switch serverConfig.Type {
		case "smtp":
			srv := forwarder.NewSMTPServer()
			handler, _err = srv.Configure(ctx, serverConfig.Settings)
		case "ses":
			srv := forwarder.NewSESServer()
			handler, _err = srv.Configure(ctx, serverConfig.Settings)
		case "log":
			srv := forwarder.NewLogServer()
			handler, err = srv.Configure(ctx, serverConfig.Settings)
		default:
			_err = fmt.Errorf("unrecognized server type: %s. allowed values: smtp, ses, log", serverConfig.Type)
		}

		if _err != nil {
			err = multierror.Append(err, _err)
			continue
		}

		reg.AddForwarder(handler, serverConfig.Weight)
	}

	if reg.Len() <= 0 {
		multierror.Append(err, errorEmptyRegistry)
	}

	return reg, err
}
