package main

import (
	"net"
	"sync"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
)

// MultiHostZone is an mDNS zone that supports multiple DNS record types
type MultiHostZone struct {
	ip        net.IP
	ipv6      *net.IP
	hostnames *set.Set[fqdn]
	records   map[string]map[uint16][]dns.RR
	mu        sync.RWMutex
}

var _ mdns.Zone = (*MultiHostZone)(nil)

func newMultiHostZone(ip net.IP, hostnames []string) *MultiHostZone {
	return &MultiHostZone{
		ip: ip,
		hostnames: set.FromFunc(hostnames, func(h string) fqdn {
			return fqdn(dns.Fqdn(h))
		}),
		records: make(map[string]map[uint16][]dns.RR),
	}
}

// Sync updates the hostnames list and clears additional records
func (z *MultiHostZone) Sync(hostnames []string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	z.hostnames = set.FromFunc(hostnames, func(h string) fqdn {
		return fqdn(dns.Fqdn(h))
	})
	// Clear additional records when syncing
	z.records = make(map[string]map[uint16][]dns.RR)
}

// SetIPv6 sets the IPv6 address for AAAA records
func (z *MultiHostZone) SetIPv6(ip net.IP) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.ipv6 = &ip
}

// AddAAAARecord adds an AAAA (IPv6) record for a hostname
func (z *MultiHostZone) AddAAAARecord(hostname string, ip net.IP) {
	z.mu.Lock()
	defer z.mu.Unlock()

	fqdnName := fqdn(dns.Fqdn(hostname))
	if z.records[string(fqdnName)] == nil {
		z.records[string(fqdnName)] = make(map[uint16][]dns.RR)
	}

	z.records[string(fqdnName)][dns.TypeAAAA] = append(z.records[string(fqdnName)][dns.TypeAAAA],
		&dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   string(fqdnName),
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    120,
			},
			AAAA: ip,
		})
}

// AddCNAMERecord adds a CNAME record for a hostname
func (z *MultiHostZone) AddCNAMERecord(hostname string, target string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	fqdnName := fqdn(dns.Fqdn(hostname))
	if z.records[string(fqdnName)] == nil {
		z.records[string(fqdnName)] = make(map[uint16][]dns.RR)
	}

	z.records[string(fqdnName)][dns.TypeCNAME] = append(z.records[string(fqdnName)][dns.TypeCNAME],
		&dns.CNAME{
			Hdr: dns.RR_Header{
				Name:   string(fqdnName),
				Rrtype: dns.TypeCNAME,
				Class:  dns.ClassINET,
				Ttl:    120,
			},
			Target: dns.Fqdn(target),
		})
}

// AddTXTRecord adds a TXT record for a hostname
func (z *MultiHostZone) AddTXTRecord(hostname string, txt []string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	fqdnName := fqdn(dns.Fqdn(hostname))
	if z.records[string(fqdnName)] == nil {
		z.records[string(fqdnName)] = make(map[uint16][]dns.RR)
	}

	z.records[string(fqdnName)][dns.TypeTXT] = append(z.records[string(fqdnName)][dns.TypeTXT],
		&dns.TXT{
			Hdr: dns.RR_Header{
				Name:   string(fqdnName),
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    120,
			},
			Txt: txt,
		})
}

// AddSRVRecord adds an SRV record for a hostname
func (z *MultiHostZone) AddSRVRecord(hostname string, priority, weight, port uint16, target string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	fqdnName := fqdn(dns.Fqdn(hostname))
	if z.records[string(fqdnName)] == nil {
		z.records[string(fqdnName)] = make(map[uint16][]dns.RR)
	}

	z.records[string(fqdnName)][dns.TypeSRV] = append(z.records[string(fqdnName)][dns.TypeSRV],
		&dns.SRV{
			Hdr: dns.RR_Header{
				Name:   string(fqdnName),
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
				Ttl:    120,
			},
			Priority: priority,
			Weight:   weight,
			Port:     port,
			Target:   dns.Fqdn(target),
		})
}

func (z *MultiHostZone) Records(q dns.Question) []dns.RR {
	if q.Qclass != dns.ClassINET {
		return nil
	}

	z.mu.RLock()
	defer z.mu.RUnlock()

	// Check if hostname is registered
	if !z.hostnames.Contains(fqdn(q.Name)) {
		return nil
	}

	// Handle A records (IPv4)
	if q.Qtype == dns.TypeA {
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

	// Handle AAAA records (IPv6)
	if q.Qtype == dns.TypeAAAA {
		if z.ipv6 != nil {
			return []dns.RR{
				&dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    120,
					},
					AAAA: *z.ipv6,
				},
			}
		}
		// Check for additional AAAA records
		if records, ok := z.records[q.Name][dns.TypeAAAA]; ok && len(records) > 0 {
			return records
		}
		return nil
	}

	// Handle additional record types
	if records, ok := z.records[q.Name][q.Qtype]; ok && len(records) > 0 {
		return records
	}

	return nil
}
