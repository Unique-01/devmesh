package proxy

import (
	"testing"
)

func TestDNSServerResolution(t *testing.T) {
	reg := NewRouteRegistry()
	_ = reg.AddRoute("vault.local.dev", "http://localhost:3000")

	// Start DNS server on random UDP port
	dnsServer := NewDNSServer("127.0.0.1:0", reg)
	if dnsServer == nil {
		t.Fatalf("expected non-nil dnsServer")
	}
}

func TestDNSServerQueryHandling(t *testing.T) {
	reg := NewRouteRegistry()
	_ = reg.AddRoute("api.local.dev", "http://localhost:4000")

	dnsServer := NewDNSServer("127.0.0.1:0", reg)

	// Construct a manual DNS query packet for api.local.dev
	// Header: ID (2 bytes), Flags (2 bytes), QDCOUNT=1, ANCOUNT=0, NSCOUNT=0, ARCOUNT=0
	query := []byte{
		0xAB, 0xCD, // ID
		0x01, 0x00, // Flags (recursion desired)
		0x00, 0x01, // QDCOUNT = 1
		0x00, 0x00, // ANCOUNT = 0
		0x00, 0x00, // NSCOUNT = 0
		0x00, 0x00, // ARCOUNT = 0
	}

	// QNAME: api.local.dev -> 3 api 5 local 3 dev 0
	query = append(query, 3)
	query = append(query, "api"...)
	query = append(query, 5)
	query = append(query, "local"...)
	query = append(query, 3)
	query = append(query, "dev"...)
	query = append(query, 0)

	// QTYPE = A (1), QCLASS = IN (1)
	query = append(query, 0, 1, 0, 1)

	resp := dnsServer.handleDNSQuery(query)
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}

	// Verify Transaction ID
	if resp[0] != 0xAB || resp[1] != 0xCD {
		t.Errorf("expected transaction ID ABCD, got %X%X", resp[0], resp[1])
	}

	// Verify ANCOUNT = 1 (Answer present)
	anCount := uint16(resp[6])<<8 | uint16(resp[7])
	if anCount != 1 {
		t.Errorf("expected ANCOUNT = 1, got %d", anCount)
	}

	// Test unknown domain with wildcard/fallback rule
	queryUnknown := []byte{
		0x12, 0x34,
		0x01, 0x00,
		0x00, 0x01,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
	}
	// unknown.other
	queryUnknown = append(queryUnknown, 7)
	queryUnknown = append(queryUnknown, "unknown"...)
	queryUnknown = append(queryUnknown, 5)
	queryUnknown = append(queryUnknown, "other"...)
	queryUnknown = append(queryUnknown, 0)
	queryUnknown = append(queryUnknown, 0, 1, 0, 1)

	respUnknown := dnsServer.handleDNSQuery(queryUnknown)
	// ANCOUNT should be 0, RCODE should be NXDomain (3)
	anCountUnknown := uint16(respUnknown[6])<<8 | uint16(respUnknown[7])
	if anCountUnknown != 0 {
		t.Errorf("expected ANCOUNT = 0 for unknown domain, got %d", anCountUnknown)
	}

	rcode := respUnknown[3] & 0x0F
	if rcode != 3 {
		t.Errorf("expected RCODE = 3 (NXDomain), got %d", rcode)
	}
}
