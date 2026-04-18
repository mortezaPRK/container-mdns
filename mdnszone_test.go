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

func TestMultiHostZone_AAAARecords(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	ipv6 := net.ParseIP("2001:db8::1")
	zone := newMultiHostZone(ip, []string{"test.local"})

	// Set IPv6 address
	zone.SetIPv6(ipv6)

	// Query for AAAA record
	q := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 1, "Expected 1 AAAA record")

	rec, ok := records[0].(*dns.AAAA)
	req.True(ok, "Expected *dns.AAAA, got %T", records[0])
	req.Equal(ipv6.String(), rec.AAAA.String(), "Expected IPv6 %s", ipv6)
}

func TestMultiHostZone_AdditionalAAAARecords(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"test.local"})

	// Add additional AAAA record
	ipv6 := net.ParseIP("2001:db8::2")
	zone.AddAAAARecord("test.local", ipv6)

	// Query for AAAA record
	q := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 1, "Expected 1 additional AAAA record")

	rec, ok := records[0].(*dns.AAAA)
	req.True(ok, "Expected *dns.AAAA, got %T", records[0])
	req.Equal(ipv6.String(), rec.AAAA.String(), "Expected IPv6 %s", ipv6)
}

func TestMultiHostZone_CNAMERecords(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"alias.local"})

	// Add CNAME record
	zone.AddCNAMERecord("alias.local", "target.local")

	// Query for CNAME record
	q := dns.Question{Name: dns.Fqdn("alias.local"), Qtype: dns.TypeCNAME, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 1, "Expected 1 CNAME record")

	rec, ok := records[0].(*dns.CNAME)
	req.True(ok, "Expected *dns.CNAME, got %T", records[0])
	req.Equal("target.local.", rec.Target, "Expected target to be target.local.")
}

func TestMultiHostZone_TXTRecords(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"test.local"})

	// Add TXT record
	txtData := []string{"service=test", "version=1.0"}
	zone.AddTXTRecord("test.local", txtData)

	// Query for TXT record
	q := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeTXT, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 1, "Expected 1 TXT record")

	rec, ok := records[0].(*dns.TXT)
	req.True(ok, "Expected *dns.TXT, got %T", records[0])
	req.Equal(txtData, rec.Txt, "Expected TXT data %v", txtData)
}

func TestMultiHostZone_SRVRecords(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"_http._tcp.local"})

	// Add SRV record
	zone.AddSRVRecord("_http._tcp.local", 10, 60, 8080, "backend.local")

	// Query for SRV record
	q := dns.Question{Name: dns.Fqdn("_http._tcp.local"), Qtype: dns.TypeSRV, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 1, "Expected 1 SRV record")

	rec, ok := records[0].(*dns.SRV)
	req.True(ok, "Expected *dns.SRV, got %T", records[0])
	req.Equal(uint16(10), rec.Priority, "Expected priority 10")
	req.Equal(uint16(60), rec.Weight, "Expected weight 60")
	req.Equal(uint16(8080), rec.Port, "Expected port 8080")
	req.Equal("backend.local.", rec.Target, "Expected target backend.local.")
}

func TestMultiHostZone_MultipleRecordTypesSameHost(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	ipv6 := net.ParseIP("2001:db8::1")
	zone := newMultiHostZone(ip, []string{"test.local"})

	// Set IPv6
	zone.SetIPv6(ipv6)

	// Add additional records
	zone.AddTXTRecord("test.local", []string{"key=value"})
	zone.AddSRVRecord("test.local", 0, 0, 443, "target.local")

	// Query for A record
	qA := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeA, Qclass: dns.ClassINET}
	recordsA := zone.Records(qA)
	req.Len(recordsA, 1, "Expected 1 A record")
	_, ok := recordsA[0].(*dns.A)
	req.True(ok, "Expected *dns.A")

	// Query for AAAA record
	qAAAA := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}
	recordsAAAA := zone.Records(qAAAA)
	req.Len(recordsAAAA, 1, "Expected 1 AAAA record")
	_, ok = recordsAAAA[0].(*dns.AAAA)
	req.True(ok, "Expected *dns.AAAA")

	// Query for TXT record
	qTXT := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeTXT, Qclass: dns.ClassINET}
	recordsTXT := zone.Records(qTXT)
	req.Len(recordsTXT, 1, "Expected 1 TXT record")
	_, ok = recordsTXT[0].(*dns.TXT)
	req.True(ok, "Expected *dns.TXT")

	// Query for SRV record
	qSRV := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeSRV, Qclass: dns.ClassINET}
	recordsSRV := zone.Records(qSRV)
	req.Len(recordsSRV, 1, "Expected 1 SRV record")
	_, ok = recordsSRV[0].(*dns.SRV)
	req.True(ok, "Expected *dns.SRV")
}

func TestMultiHostZone_UnsupportedRecordType(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"test.local"})

	// Query for MX record (not supported)
	q := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeMX, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 0, "Expected 0 records for unsupported MX type")
}

func TestMultiHostZone_SyncClearsAdditionalRecords(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"test.local"})

	// Add additional records
	ipv6 := net.ParseIP("2001:db8::1")
	zone.SetIPv6(ipv6)
	zone.AddTXTRecord("test.local", []string{"data"})
	zone.AddSRVRecord("test.local", 0, 0, 80, "target.local")

	// Verify TXT record exists
	qTXT := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeTXT, Qclass: dns.ClassINET}
	recordsTXT := zone.Records(qTXT)
	req.Len(recordsTXT, 1, "Expected 1 TXT record before sync")

	// Sync with same hostnames
	zone.Sync([]string{"test.local"})

	// Verify additional records are cleared
	recordsTXT = zone.Records(qTXT)
	req.Len(recordsTXT, 0, "Expected 0 TXT records after sync")

	// Verify A record still works
	qA := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeA, Qclass: dns.ClassINET}
	recordsA := zone.Records(qA)
	req.Len(recordsA, 1, "Expected 1 A record after sync")
}

func TestMultiHostZone_AddMultipleRecordsSameType(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"test.local"})

	// Add multiple TXT records
	zone.AddTXTRecord("test.local", []string{"record1"})
	zone.AddTXTRecord("test.local", []string{"record2"})

	// Query for TXT records
	q := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeTXT, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 2, "Expected 2 TXT records")
}

func TestMultiHostZone_QueryNonExistentHostname(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"exists.local"})

	// Add records to existing hostname
	zone.AddTXTRecord("exists.local", []string{"data"})

	// Query for non-existent hostname with various types
	hostnames := []string{"missing.local", "another.local"}
	types := []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeTXT, dns.TypeSRV}

	for _, hostname := range hostnames {
		for _, qtype := range types {
			q := dns.Question{Name: dns.Fqdn(hostname), Qtype: qtype, Qclass: dns.ClassINET}
			records := zone.Records(q)
			req.Len(records, 0, "Expected 0 records for %s type %d", hostname, qtype)
		}
	}
}

func TestMultiHostZone_EmptyHostnameList(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{})

	// Query for A record
	q := dns.Question{Name: dns.Fqdn("test.local"), Qtype: dns.TypeA, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 0, "Expected 0 records for empty hostname list")
}

func TestMultiHostZone_CNAMEWithFQDN(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"alias.local"})

	// Add CNAME with FQDN target
	zone.AddCNAMERecord("alias.local", "target.local")

	// Query for CNAME record
	q := dns.Question{Name: dns.Fqdn("alias.local"), Qtype: dns.TypeCNAME, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 1, "Expected 1 CNAME record")

	rec, ok := records[0].(*dns.CNAME)
	req.True(ok, "Expected *dns.CNAME, got %T", records[0])
	// Target should be FQDN (ending with dot)
	req.Equal("target.local.", rec.Target, "Expected target to be fully qualified")
}

func TestMultiHostZone_SRVDefaultValues(t *testing.T) {
	req := require.New(t)
	ip := net.ParseIP("192.168.1.100")
	zone := newMultiHostZone(ip, []string{"service.local"})

	// Add SRV with default values
	zone.AddSRVRecord("service.local", 0, 0, 0, "target.local")

	// Query for SRV record
	q := dns.Question{Name: dns.Fqdn("service.local"), Qtype: dns.TypeSRV, Qclass: dns.ClassINET}
	records := zone.Records(q)
	req.Len(records, 1, "Expected 1 SRV record")

	rec, ok := records[0].(*dns.SRV)
	req.True(ok, "Expected *dns.SRV, got %T", records[0])
	req.Equal(uint16(0), rec.Priority, "Expected priority 0")
	req.Equal(uint16(0), rec.Weight, "Expected weight 0")
	req.Equal(uint16(0), rec.Port, "Expected port 0")
}

