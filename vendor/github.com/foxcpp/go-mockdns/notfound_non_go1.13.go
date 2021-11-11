//+build !go1.13

package mockdns

import "net"

func notFound(host string) error {
	return &net.DNSError{
		Err:        "no such host",
		Name:       host,
		Server:     "127.0.0.1:53",
	}
}

func isNotFound(dnsErr *net.DNSError) bool {
    return dnsErr.Err == "no such host"
}
