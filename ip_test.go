package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetIPFromEnvOrDefault_EnvSet(t *testing.T) {
	req := require.New(t)
	_ = os.Setenv("HOSTNAME_IP", "1.2.3.4")
	defer func() { _ = os.Unsetenv("HOSTNAME_IP") }()
	ip, err := getIPFromEnvOrDefault("")
	req.NoError(err)
	req.Equal("1.2.3.4", ip)
}

func TestGetIPFromEnvOrDefault_Hostname(t *testing.T) {
	req := require.New(t)
	_ = os.Unsetenv("HOSTNAME_IP")
	// Monkey patch os.Hostname for test
	orig := osHostname
	osHostname = func() (string, error) { return "myhost", nil }
	defer func() { osHostname = orig }()

	ip, err := getIPFromEnvOrDefault("")
	req.NoError(err)
	req.Equal("myhost", ip)
}

func TestGetIPFromEnvOrDefault_HostnameError(t *testing.T) {
	req := require.New(t)
	_ = os.Unsetenv("HOSTNAME_IP")
	orig := osHostname
	osHostname = func() (string, error) { return "", os.ErrNotExist }
	defer func() { osHostname = orig }()

	_, err := getIPFromEnvOrDefault("")
	req.Error(err)
}

func TestGetIPFromEnvOrDefault_DefaultIP(t *testing.T) {
	req := require.New(t)
	_ = os.Unsetenv("HOSTNAME_IP")

	ip, err := getIPFromEnvOrDefault("192.168.1.1")
	req.NoError(err)
	req.Equal("192.168.1.1", ip)
}

func TestGetIPFromEnvOrDefault_DefaultIPOverridesEnv(t *testing.T) {
	req := require.New(t)
	_ = os.Setenv("HOSTNAME_IP", "1.2.3.4")
	defer func() { _ = os.Unsetenv("HOSTNAME_IP") }()

	ip, err := getIPFromEnvOrDefault("192.168.1.1")
	req.NoError(err)
	req.Equal("192.168.1.1", ip)
}
