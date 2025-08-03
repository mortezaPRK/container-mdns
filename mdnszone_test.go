package main

import (
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestMultiHostZone_SyncAndRecords(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{})

	hosts := []string{"foo.local", "bar.local"}
	zone.Sync(hosts)

	for _, h := range hosts {
		fqdn := dns.Fqdn(h)
		q := dns.Question{Name: fqdn, Qtype: dns.TypeA, Qclass: dns.ClassINET}
		records := zone.Records(q)
		req.Len(records, 1, "Expected 1 record for %s", h)
		rec, ok := records[0].(*dns.A)
		req.True(ok, "Expected *dns.A for %s, got %T", h, records[0])
		req.Equal(ip.String(), rec.A.String(), "Expected IP %s for %s", ip, h)
	}

	// Query for a non-existent host
	q := dns.Question{Name: dns.Fqdn("baz.local"), Qtype: dns.TypeA, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 0, "Expected 0 records for baz.local")
}

func TestMultiHostZone_RecordsWrongTypeOrClass(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("10.0.0.1")
	zone := newMultiHostZone(ip, []string{"foo.local"})

	q := dns.Question{Name: dns.Fqdn("foo.local"), Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}
	req.Len(zone.Records(q), 0, "Expected 0 records for AAAA query")

	q = dns.Question{Name: dns.Fqdn("foo.local"), Qtype: dns.TypeA, Qclass: dns.ClassCHAOS}
	req.Len(zone.Records(q), 0, "Expected 0 records for CHAOS class")
}
