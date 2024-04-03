package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/creasty/defaults"
	"github.com/hashicorp/go-multierror"
	yaml "gopkg.in/yaml.v2"
)

var (
	_emptyConfig            = Config{}
	errEmptyFile            = errors.New("empty yaml file contents")
	errEmptyUpstreamServers = errors.New("no specified upstream servers, supported: smtp, ses, log")
)

// Config represents the structure of the yaml file.
type Config struct {
	ServerConfig ProxyServerConfig `yaml:"smtpd-proxy"`
}

// ProxyServerConfig the top level config.
type ProxyServerConfig struct {
	Listen                string           `default:"127.0.0.1:1025" yaml:"listen"`
	Ehlo                  string           `default:"-"              yaml:"ehlo"`
	Username              string           `default:"-"              yaml:"username"`
	Password              string           `default:"-"              yaml:"password"`
	IsAnonAuthAllowed     bool             `default:"-"              yaml:"is_anon_auth_allowed"`
	ServerCertificatePath string           `default:"-"              yaml:"server-cert"`
	ServerKeyPath         string           `default:"-"              yaml:"server-key"`
	UpstreamServers       []UpstreamServer `yaml:"upstream-servers"`
}

// UpstreamServer upstream server config.
type UpstreamServer struct {
	Type     string         `default:"smtp" yaml:"type"`
	Weight   int            `default:"1"    yaml:"weight"`
	Settings map[string]any `default:"{}"   yaml:"settings"`
}

// Parse takes a raw data and returns Config.
func Parse(reader io.Reader) (*Config, error) {
	var c Config

	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(bytes, &c); err != nil {
		return nil, err
	}

	if reflect.DeepEqual(c, _emptyConfig) {
		return nil, errEmptyFile
	}

	return &c, nil
}

// ParseFile takes a path to a yaml file and produces a parsed Config.
func ParseFile(path string) (*Config, error) {
	data, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	return Parse(data)
}

// LoadDefaults sets defaults for configuration.
func (c *Config) LoadDefaults() (*Config, error) {
	if err := defaults.Set(c); err != nil {
		return nil, err
	}

	var err error
	for _, server := range c.ServerConfig.UpstreamServers {
		var _castErr error
		if server.Weight <= 0 {
			err = multierror.Append(err, fmt.Errorf("invalid non-positive weight: %v", server.Weight))
		}
		switch server.Type {
		case "smtp":
		case "ses":
		case "log":
		default:
			_castErr = fmt.Errorf("unrecognized server type: %s. allowed values: smtp, ses, log", server.Type)
		}
		if _castErr != nil {
			err = multierror.Append(err, _castErr)
		}
	}

	if len(c.ServerConfig.UpstreamServers) == 0 {
		return nil, errEmptyUpstreamServers
	}

	if err != nil {
		return nil, err
	}

	return c, nil
}
