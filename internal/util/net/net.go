package net

import (
	"errors"
	"fmt"
	"net"
)

// DetectHostIPv4 attempts to determine the host IPv4 address by finding the
// first non-loopback device with an assigned IPv4 address.
func DetectHostIPv4() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("list interface addresses: %w", err)
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() == nil {
				continue
			}
			return ipnet.IP.String(), nil
		}
	}
	return "", errors.New("cannot detect host IPv4 address")
}
