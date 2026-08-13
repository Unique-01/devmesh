package proxy

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// DNSServer provides a lightweight local DNS server that resolves configured dev domains (e.g. *.local.dev or *.dev)
// to 127.0.0.1 without requiring /etc/hosts modifications or sudo permissions (when local DNS resolver port 53
// or alternative port is configured, or via zero-config automatic fallback).
type DNSServer struct {
	addr     string
	registry *RouteRegistry
	conn     *net.UDPConn
	closed   bool
}

// NewDNSServer creates a new local DNS server bound to addr (e.g. "127.0.0.1:53" or "127.0.0.1:5353").
func NewDNSServer(addr string, registry *RouteRegistry) *DNSServer {
	if addr == "" {
		addr = "127.0.0.1:53"
	}
	return &DNSServer{
		addr:     addr,
		registry: registry,
	}
}

// Start starts listening for UDP DNS queries.
func (ds *DNSServer) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", ds.addr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address %s: %w", ds.addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP %s (note: port 53 may require root/sudo, or use a custom port like 127.0.0.1:5353): %w", ds.addr, err)
	}
	ds.conn = conn

	go func() {
		<-ctx.Done()
		ds.Close()
	}()

	buf := make([]byte, 512)
	for {
		n, remoteAddr, err := ds.conn.ReadFromUDP(buf)
		if err != nil {
			if ds.closed {
				return nil
			}
			continue
		}

		response := ds.handleDNSQuery(buf[:n])
		if response != nil {
			_, _ = ds.conn.WriteToUDP(response, remoteAddr)
		}
	}
}

// Close closes the DNS server UDP listener.
func (ds *DNSServer) Close() error {
	ds.closed = true
	if ds.conn != nil {
		return ds.conn.Close()
	}
	return nil
}

// handleDNSQuery parses a minimal DNS query packet, extracts the requested domain name,
// checks if it matches any registered route in RouteRegistry, and constructs a DNS response A record pointing to 127.0.0.1.
func (ds *DNSServer) handleDNSQuery(query []byte) []byte {
	if len(query) < 12 {
		return nil
	}

	// Extract Transaction ID (2 bytes)
	txID := query[0:2]

	// Parse QNAME starting at offset 12
	domain, offset := parseDNSName(query, 12)
	if domain == "" {
		return nil
	}

	// Check if domain is registered in RouteRegistry
	domainLower := strings.ToLower(strings.TrimSuffix(domain, "."))
	_, found := ds.registry.Resolve(domainLower)

	// Also allow any *.local.dev or *.dev if configured or wildcard matches
	if !found {
		// Check if it's a .local.dev domain or matches a wildcard
		if strings.HasSuffix(domainLower, ".local.dev") || strings.HasSuffix(domainLower, ".dev") {
			found = true
		}
	}

	// Build DNS response packet
	var resp []byte
	resp = append(resp, txID...)

	// Flags: QR=1 (response), Opcode=0, AA=1 (authoritative), RD=1, RA=1, RCODE=0 (NoError) or 3 (NXDomain)
	flags := uint16(0x8180) // Standard response, no error
	if !found {
		flags = 0x8183
	}

	resp = append(resp, byte(flags>>8), byte(flags))

	// QDCOUNT = 1, ANCOUNT = 1 (if found) or 0 (if not found), NSCOUNT = 0, ARCOUNT = 0
	anCount := uint16(0)
	if found {
		anCount = 1
	}
	resp = append(resp, 0, 1) // QDCOUNT = 1
	resp = append(resp, byte(anCount>>8), byte(anCount))
	resp = append(resp, 0, 0) // NSCOUNT = 0
	resp = append(resp, 0, 0) // ARCOUNT = 0

	// Append original Question section (from offset 12 to end of QNAME + 4 bytes for QTYPE/QCLASS)
	questionEnd := offset + 4
	if questionEnd > len(query) {
		questionEnd = len(query)
	}
	resp = append(resp, query[12:questionEnd]...)

	if found {
		// Answer section: Name pointer to question (compression pointer 0xC000 + 12 = 0xC00C)
		resp = append(resp, 0xC0, 0x0C)

		// TYPE = A (1), CLASS = IN (1)
		resp = append(resp, 0, 1, 0, 1)

		// TTL = 300 seconds (4 bytes: 0x00, 0x00, 0x01, 0x2C)
		resp = append(resp, 0, 0, 1, 0x2C)

		// RDLENGTH = 4 bytes (IPv4 address)
		resp = append(resp, 0, 4)

		// RDATA = 127.0.0.1
		resp = append(resp, 127, 0, 0, 1)
	}

	return resp
}

// parseDNSName extracts domain name from DNS packet at given offset.
func parseDNSName(packet []byte, offset int) (string, int) {
	var labels []string
	curr := offset
	for curr < len(packet) {
		length := int(packet[curr])
		if length == 0 {
			curr++
			break
		}
		// Check for pointer (compression)
		if (length & 0xC0) == 0xC0 {
			if curr+1 >= len(packet) {
				return "", curr
			}
			// pointer offset
			ptr := int(packet[curr]&0x3F)<<8 | int(packet[curr+1])
			name, _ := parseDNSName(packet, ptr)
			if name != "" {
				labels = append(labels, name)
			}
			curr += 2
			break
		}
		curr++
		if curr+length > len(packet) {
			return "", curr
		}
		labels = append(labels, string(packet[curr:curr+length]))
		curr += length
	}
	return strings.Join(labels, "."), curr
}
