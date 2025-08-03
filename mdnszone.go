package main

import (
	"net"
	"sync"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
)

type MultiHostZone struct {
	ip        net.IP
	hostnames *set.Set[fqdn]
	mu        sync.RWMutex
}

var _ mdns.Zone = (*MultiHostZone)(nil)

func newMultiHostZone(ip net.IP, hostnames []string) *MultiHostZone {
	return &MultiHostZone{
		ip: ip,
		hostnames: set.FromFunc(hostnames, func(h string) fqdn {
			return fqdn(dns.Fqdn(h))
		}),
	}
}

func (z *MultiHostZone) Sync(hostnames []string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	z.hostnames = set.FromFunc(hostnames, func(h string) fqdn {
		return fqdn(dns.Fqdn(h))
	})
}

func (z *MultiHostZone) Records(q dns.Question) []dns.RR {
	if q.Qclass != dns.ClassINET || q.Qtype != dns.TypeA {
		return nil
	}

	z.mu.RLock()
	defer z.mu.RUnlock()

	if !z.hostnames.Contains(fqdn(q.Name)) {
		return nil
	}

	return []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    120,
			},
			A: z.ip,
		},
	}
}
