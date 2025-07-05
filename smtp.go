package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/emersion/go-smtp"
)

// SMTP Configuration
const (
	DefaultSMTPHost = "0.0.0.0"
	DefaultSMTPPort = 2525
	SMTPDomain      = "localhost"
	ReadTimeout     = 10 * time.Second
	WriteTimeout    = 10 * time.Second
	MaxMessageBytes = 1024 * 1024 // 1MB
	MaxRecipients   = 50
)

// SMTPServer wraps the SMTP server functionality
type SMTPServer struct {
	server          *smtp.Server
	emailProcessor  *EmailProcessor
	listenAddr      string
	allowedNetworks []*net.IPNet
	tlsConfig       *tls.Config
}

// NewSMTPServer creates a new SMTP server instance
func NewSMTPServer(emailProcessor *EmailProcessor, listenHost string, port int, allowedNetworks []string, tlsConfig *tls.Config) *SMTPServer {
	if listenHost == "" {
		listenHost = DefaultSMTPHost
	}
	if port == 0 {
		port = DefaultSMTPPort
	}

	// Parse allowed networks
	var ipNets []*net.IPNet
	for _, network := range allowedNetworks {
		if network != "" {
			_, ipNet, err := net.ParseCIDR(network)
			if err != nil {
				log.Printf("Warning: invalid CIDR network '%s': %v", network, err)
				continue
			}
			ipNets = append(ipNets, ipNet)
			log.Printf("Added allowed network: %s", network)
		}
	}

	smtpServer := &SMTPServer{
		listenAddr:      fmt.Sprintf("%s:%d", listenHost, port),
		emailProcessor:  emailProcessor,
		allowedNetworks: ipNets,
		tlsConfig:       tlsConfig,
	}

	backend := &SMTPBackend{
		EmailProcessor:  emailProcessor,
		AllowedNetworks: ipNets,
	}

	server := smtp.NewServer(backend)
	server.Addr = smtpServer.listenAddr
	server.Domain = SMTPDomain
	server.ReadTimeout = ReadTimeout
	server.WriteTimeout = WriteTimeout
	server.MaxMessageBytes = MaxMessageBytes
	server.MaxRecipients = MaxRecipients
	server.AllowInsecureAuth = true

	// Configure TLS if provided
	if tlsConfig != nil {
		server.TLSConfig = tlsConfig
		log.Printf("TLS/STARTTLS enabled for SMTP server")
	} else {
		log.Printf("TLS disabled - plain SMTP only")
	}

	smtpServer.server = server
	return smtpServer
}

// Start starts the SMTP server
func (s *SMTPServer) Start() error {
	log.Printf("Starting SMTP server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop stops the SMTP server
func (s *SMTPServer) Stop() error {
	log.Println("Stopping SMTP server...")
	return s.server.Close()
}

// SMTPBackend implements the SMTP backend interface
type SMTPBackend struct {
	EmailProcessor  *EmailProcessor
	AllowedNetworks []*net.IPNet
}

// isIPAllowed checks if an IP address is in the allowed networks
func (sb *SMTPBackend) isIPAllowed(remoteAddr string) bool {
	// If no networks specified, allow all
	if len(sb.AllowedNetworks) == 0 {
		return true
	}

	// Extract IP from address (remove port)
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// If no port, use the address as-is
		host = remoteAddr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		log.Printf("Warning: could not parse IP address: %s", host)
		return false
	}

	// Check against allowed networks
	for _, network := range sb.AllowedNetworks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// NewSession creates a new SMTP session
func (sb *SMTPBackend) NewSession(conn *smtp.Conn) (smtp.Session, error) {
	remoteAddr := conn.Conn().RemoteAddr().String()

	// Check IP ACL if configured
	if !sb.isIPAllowed(remoteAddr) {
		log.Printf("Connection rejected from %s (not in allowed networks)", remoteAddr)
		return nil, fmt.Errorf("connection not allowed from %s", remoteAddr)
	}

	log.Printf("New SMTP session from: %s", remoteAddr)
	return &SMTPSession{
		EmailProcessor: sb.EmailProcessor,
		RemoteAddr:     remoteAddr,
	}, nil
}

// SMTPSession represents an active SMTP session
type SMTPSession struct {
	EmailProcessor *EmailProcessor
	From           string
	To             []string
	RemoteAddr     string
}

// AuthPlain handles PLAIN authentication
func (s *SMTPSession) AuthPlain(username, password string) error {
	log.Printf("SMTP Auth attempt - Username: %s", username)
	// Accept any authentication for simplicity
	// In production, you might want to implement proper authentication
	return nil
}

// Mail handles the MAIL FROM command
func (s *SMTPSession) Mail(from string, opts *smtp.MailOptions) error {
	log.Printf("MAIL FROM: %s", from)
	s.From = from
	return nil
}

// Rcpt handles the RCPT TO command
func (s *SMTPSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	log.Printf("RCPT TO: %s", to)
	s.To = append(s.To, to)
	return nil
}

// Data handles the email data transmission
func (s *SMTPSession) Data(r io.Reader) error {
	log.Printf("Receiving email data from %s to %v (remote: %s)", s.From, s.To, s.RemoteAddr)

	// Read all email data
	data, err := io.ReadAll(r)
	if err != nil {
		log.Printf("Error reading email data: %v", err)
		return fmt.Errorf("failed to read email data: %w", err)
	}

	log.Printf("Received %d bytes of email data", len(data))

	// Process the email through the email processor
	if err := s.EmailProcessor.ProcessEmail(data, s.From, s.To, s.RemoteAddr); err != nil {
		log.Printf("Error processing email: %v", err)
		return fmt.Errorf("failed to process email: %w", err)
	}

	log.Println("Email successfully processed and forwarded")
	return nil
}

// Reset resets the session state
func (s *SMTPSession) Reset() {
	log.Println("SMTP session reset")
	s.From = ""
	s.To = nil
}

// Logout handles session termination
func (s *SMTPSession) Logout() error {
	log.Println("SMTP session logout")
	return nil
}

// GetServerAddress returns the server address
func (s *SMTPServer) GetServerAddress() string {
	return s.listenAddr
}
