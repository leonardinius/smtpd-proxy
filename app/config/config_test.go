package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmokeLoadConfigShouldBeOk(t *testing.T) {
	t.Parallel()
	data := `
smtpd-proxy:
  listen: 127.0.0.1:1025
  ehlo: 127.0.0.1
  username: user
  password: secret
  server-cert: server.crt
  server-key: server.key

  upstream-servers:
    - type: smtp
      weight: 25
      settings:
        address: 127.0.0.1:1026
        auth: plain
        username: user
        password: secret
    - type: ses
      weight: 25
      settings:
        aws_access_key_id: xxx
        aws_secret_access_key: yyy
        region: us_west_2
`

	c, err := Parse(strings.NewReader(data))
	require.Nil(t, err)
	require.NotNil(t, c)

	srv := c.ServerConfig
	assert.Equal(t, "127.0.0.1:1025", srv.Listen)
	assert.Equal(t, "127.0.0.1", srv.Ehlo)
	assert.Equal(t, "user", srv.Username)
	assert.Equal(t, "secret", srv.Password)
	assert.Equal(t, "server.crt", srv.ServerCertificatePath)
	assert.Equal(t, "server.key", srv.ServerKeyPath)

	assert.Equal(t, 2, len(srv.UpstreamServers))

	assert.Equal(t, "smtp", srv.UpstreamServers[0].Type)
	assert.Equal(t, 25, srv.UpstreamServers[0].Weight)
	assert.Equal(t,
		map[string]any{
			"address":  "127.0.0.1:1026",
			"auth":     "plain",
			"username": "user",
			"password": "secret"},
		srv.UpstreamServers[0].Settings)

	assert.Equal(t, "ses", srv.UpstreamServers[1].Type)
	assert.Equal(t, 25, srv.UpstreamServers[1].Weight)
	assert.Equal(t,
		map[string]any{
			"aws_access_key_id":     "xxx",
			"aws_secret_access_key": "yyy",
			"region":                "us_west_2",
		},
		srv.UpstreamServers[1].Settings)
}

func TestLoadEmptyFileShouldFail(t *testing.T) {
	t.Parallel()
	data := "\n"
	c, err := Parse(strings.NewReader(data))
	require.Nil(t, c)
	require.NotNil(t, err)
	assert.ErrorContains(t, err, "empty yaml file contents")
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()
	data := `
smtpd-proxy:
  upstream-servers:
    - type: smtp
      settings:
        api_key: key
`
	c, err := Parse(strings.NewReader(data))
	require.Nil(t, err)
	require.NotNil(t, c)

	c, err = c.LoadDefaults()
	require.Nil(t, err)
	require.NotNil(t, c)

	srv := c.ServerConfig
	assert.Equal(t, "127.0.0.1:1025", srv.Listen)
	assert.Equal(t, "", srv.Ehlo)
	assert.Equal(t, "", srv.Username)
	assert.Equal(t, "", srv.Password)
	assert.Equal(t, "", srv.ServerCertificatePath)
	assert.Equal(t, "", srv.ServerKeyPath)
}
