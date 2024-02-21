//go:build go1.18
// +build go1.18

package mockdns

import (
	"context"
	"fmt"

	"net/netip"
)

func (r *Resolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	var addrs []string
	var err error
	switch network {
	case "ip":
		addrs, err = r.LookupHost(ctx, host)
	case "ip4":
		_, addrs, err = r.lookupA(ctx, host)
	case "ip6":
		_, addrs, err = r.lookupAAAA(ctx, host)
	default:
		return nil, fmt.Errorf("unsupported network: %v", network)
	}
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, notFound(host)
	}

	parsed := make([]netip.Addr, len(addrs))
	for i, addr := range addrs {
		parsed[i], err = netip.ParseAddr(addr)
		if err != nil {
			return nil, err
		}
	}
	return parsed, nil
}
