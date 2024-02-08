package mockdns

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type Zone struct {
	// Return the specified error on any lookup using this zone.
	// For Server, non-nil value results in SERVFAIL response.
	Err error

	// When used with Server, set the Authenticated Data (AD) flag
	// in the responses.
	AD bool

	A     []string
	AAAA  []string
	TXT   []string
	PTR   []string
	CNAME string
	MX    []net.MX
	NS    []net.NS
	SRV   []net.SRV

	// Misc includes other associated zone records, they can be returned only
	// when used with Server.
	Misc map[dns.Type][]dns.RR
}

// Resolver is the struct that implements interface same as net.Resolver
// and so can be used as a drop-in replacement for it if tested code
// supports it.
type Resolver struct {
	Zones map[string]Zone

	// Don't follow CNAME in Zones for Lookup*.
	SkipCNAME bool
}

func (r *Resolver) LookupAddr(ctx context.Context, addr string) (names []string, err error) {
	arpa, err := dns.ReverseAddr(addr)
	if err != nil {
		return nil, err
	}

	rzone, ok := r.Zones[strings.ToLower(arpa)]
	if !ok {
		return nil, notFound(arpa)
	}
	if rzone.Err != nil {
		return nil, rzone.Err
	}

	names = make([]string, len(rzone.PTR))
	copy(names, rzone.PTR)
	return
}

func (r *Resolver) LookupCNAME(ctx context.Context, host string) (cname string, err error) {
	rzone, ok := r.Zones[strings.ToLower(host)]
	if !ok {
		return "", notFound(host)
	}

	return rzone.CNAME, nil
}

func (r *Resolver) LookupHost(ctx context.Context, host string) (addrs []string, err error) {
	_, addrs4, err := r.lookupA(ctx, host)
	if err != nil {
		return nil, err
	}
	_, addrs6, err := r.lookupAAAA(ctx, host)
	if err != nil {
		return nil, err
	}

	addrs = append(addrs, addrs4...)
	addrs = append(addrs, addrs6...)

	if len(addrs) == 0 {
		return nil, notFound(host)
	}

	return addrs, err
}

func (r *Resolver) targetZone(name string) (ad bool, rname string, zone Zone, err error) {
	rname = strings.ToLower(dns.Fqdn(name))
	rzone, ok := r.Zones[rname]
	if !ok {
		return false, "", Zone{}, notFound(name)
	}

	if rzone.Err != nil {
		return false, "", rzone, rzone.Err
	}

	ad = rzone.AD

	if !r.SkipCNAME {
		for rzone.CNAME != "" {
			rname = rzone.CNAME
			rzone, ok = r.Zones[rname]
			if !ok {
				return false, rname, Zone{}, notFound(rname)
			}
			if rzone.Err != nil {
				return false, "", rzone, rzone.Err
			}
			ad = ad && rzone.AD
		}
	}

	return ad, rname, rzone, nil
}

func (r *Resolver) lookupA(ctx context.Context, host string) (cname string, addrs []string, err error) {
	_, cname, rzone, err := r.targetZone(host)
	if err != nil {
		return cname, nil, err
	}

	return cname, rzone.A, nil
}

func (r *Resolver) lookupAAAA(ctx context.Context, host string) (cname string, addrs []string, err error) {
	_, cname, rzone, err := r.targetZone(host)
	if err != nil {
		return cname, nil, err
	}

	return cname, rzone.AAAA, nil
}

func (r *Resolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	addrs, err := r.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	parsed := make([]net.IPAddr, 0, len(addrs))
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			return nil, fmt.Errorf("malformed IP in records: %v", addr)
		}

		parsed = append(parsed, net.IPAddr{IP: ip})
	}

	return parsed, nil
}

func (r *Resolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
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

	parsed := make([]net.IP, len(addrs))
	for i, addr := range addrs {
		parsed[i] = net.ParseIP(addr)
	}
	return parsed, nil
}

func (r *Resolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	_, mx, err := r.lookupMX(ctx, name)
	res := make([]*net.MX, len(mx))
	copy(res, mx)
	return res, err
}

func (r *Resolver) lookupMX(ctx context.Context, name string) (string, []*net.MX, error) {
	_, cname, rzone, err := r.targetZone(name)
	if err != nil {
		return "", nil, err
	}

	out := make([]*net.MX, 0, len(rzone.MX))
	for _, mx := range rzone.MX {
		mxCpy := mx
		out = append(out, &mxCpy)
	}

	return cname, out, nil
}

func (r *Resolver) LookupNS(ctx context.Context, name string) ([]*net.NS, error) {
	_, ns, err := r.lookupNS(ctx, name)
	res := make([]*net.NS, len(ns))
	copy(res, ns)
	return res, err
}

func (r *Resolver) lookupNS(ctx context.Context, name string) (string, []*net.NS, error) {
	_, cname, rzone, err := r.targetZone(name)
	if err != nil {
		return "", nil, err
	}

	out := make([]*net.NS, 0, len(rzone.MX))
	for _, ns := range rzone.NS {
		nsCpy := ns
		out = append(out, &nsCpy)
	}

	return cname, out, nil
}

func (r *Resolver) LookupPort(ctx context.Context, network, service string) (port int, err error) {
	// TODO: Check whether it can cause problems with net.DefaultResolver hjacking.
	return net.LookupPort(network, service)
}

func (r *Resolver) LookupSRV(ctx context.Context, service, proto, name string) (cname string, addrs []*net.SRV, err error) {
	query := fmt.Sprintf("_%s._%s.%s", service, proto, name)
	return r.lookupSRV(ctx, query)
}

func (r *Resolver) lookupSRV(ctx context.Context, query string) (cname string, addrs []*net.SRV, err error) {
	_, cname, rzone, err := r.targetZone(query)
	if err != nil {
		return "", nil, err
	}

	out := make([]*net.SRV, 0, len(rzone.SRV))
	for _, srv := range rzone.SRV {
		srvCpy := srv
		out = append(out, &srvCpy)
	}

	return cname, out, nil
}

func (r *Resolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	_, txt, err := r.lookupTXT(ctx, name)
	res := make([]string, len(txt))
	copy(res, txt)
	return res, err
}

func (r *Resolver) lookupTXT(ctx context.Context, name string) (string, []string, error) {
	_, cname, rzone, err := r.targetZone(name)
	if err != nil {
		return "", nil, err
	}

	return cname, rzone.TXT, nil
}

// Dial implements the function similar to net.Dial that uses Resolver zones
// to find the the IP address to use. It is very simple and does not fully
// replicate the net.Dial behavior. Notably it does not implement Fast Fallback
// and always prefers IPv6 over IPv4.
func (r *Resolver) Dial(network, addr string) (net.Conn, error) {
	return r.DialContext(context.Background(), network, addr)
}

func (r *Resolver) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(host)
	if ip != nil {
		return net.Dial(network, addr)
	}

	_, addrs6, err := r.lookupAAAA(ctx, host)
	if err != nil {
		return nil, err
	}
	_, addrs4, err := r.lookupA(ctx, host)
	if err != nil {
		return nil, err
	}
	addrs := append(addrs6, addrs4...)

	if len(addrs) == 0 {
		return nil, notFound(host)
	}

	var lastErr error
	for _, addrTry := range addrs {
		conn, err := net.Dial(network, net.JoinHostPort(addrTry, port))
		if err != nil {
			lastErr = err
			continue
		}
		return conn, nil
	}
	return nil, lastErr
}
