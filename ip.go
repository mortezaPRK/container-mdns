package main

import "os"

var osHostname = os.Hostname

func getIPFromEnvOrDefault(defaultIP string) (string, error) {
	if defaultIP != "" {
		return defaultIP, nil
	}

	if ip := os.Getenv("HOSTNAME_IPs"); ip != "" {
		return ip, nil
	}

	addrs, err := osHostname()
	if err != nil {
		return "", err
	}

	return addrs, nil
}
