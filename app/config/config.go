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

var _emptyConfig = Config{}
var errorEmptyFile = errors.New("empty yaml file contents")
var errorEmptyUpstreamServers = errors.New("no specified upstream servers, supported: smtp, ses, log")

// Config represents the structure of the yaml file
type Config struct {
	ServerConfig ProxyServerConfig `yaml:"smtpd-proxy"`
}

// ProxyServerConfig the top level config
type ProxyServerConfig struct {
	Listen                string           `yaml:"listen" default:"127.0.0.1:1025"`
	Ehlo                  string           `yaml:"ehlo" default:"-"`
	Username              string           `yaml:"username" default:"-"`
	Password              string           `yaml:"password" default:"-"`
	IsAnonAuthAllowed     bool             `yaml:"is_anon_auth_allowed" default:"-"`
	ServerCertificatePath string           `yaml:"server-cert" default:"-"`
	ServerKeyPath         string           `yaml:"server-key" default:"-"`
	UpstreamServers       []UpstreamServer `yaml:"upstream-servers"`
}

// UpstreamServer upstream server config
type UpstreamServer struct {
	Type     string         `yaml:"type" default:"smtp"`
	Weight   int            `yaml:"weight" default:"1"`
	Settings map[string]any `yaml:"settings" default:"{}"`
}

// Parse takes a raw data and returns Config
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
		return nil, errorEmptyFile
	}

	return &c, nil
}

// ParseFile takes a path to a yaml file and produces a parsed Config
func ParseFile(path string) (*Config, error) {
	data, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	return Parse(data)
}

// LoadDefaults sets defaults for configuration
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
		return nil, errorEmptyUpstreamServers
	}

	if err != nil {
		return nil, err
	}

	return c, nil
}
