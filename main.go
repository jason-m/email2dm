package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

// Config holds application configuration
type Config struct {
	TelegramBotToken string
	SlackBotToken    string
	SMTPListenHost   string
	SMTPListenPort   int
	AllowedNetworks  []string
	TLSEnable        bool
	TLSCertPath      string
	TLSKeyPath       string
}

// loadConfig loads configuration from environment variables
func loadConfig() (*Config, error) {
	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	slackBotToken := os.Getenv("SLACK_BOT_TOKEN")
	smtpHost := os.Getenv("SMTP_LISTEN_HOST")
	smtpPortStr := os.Getenv("SMTP_LISTEN_PORT")
	allowedNetworksStr := os.Getenv("ALLOWED_NETWORKS")
	tlsEnableStr := os.Getenv("TLS_ENABLE")
	tlsCertPath := os.Getenv("TLS_CERT_PATH")
	tlsKeyPath := os.Getenv("TLS_KEY_PATH")

	// At least one platform token is required
	if telegramBotToken == "" && slackBotToken == "" {
		return nil, fmt.Errorf("at least one platform token is required (TELEGRAM_BOT_TOKEN or SLACK_BOT_TOKEN)")
	}

	// Default to 0.0.0.0 if not specified
	if smtpHost == "" {
		smtpHost = "0.0.0.0"
	}

	// Parse SMTP port
	smtpPort := 2525 // default
	if smtpPortStr != "" {
		port, err := strconv.Atoi(smtpPortStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SMTP_LISTEN_PORT '%s': %w", smtpPortStr, err)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("SMTP_LISTEN_PORT must be between 1 and 65535, got %d", port)
		}
		smtpPort = port
	}

	// Parse allowed networks
	var allowedNetworks []string
	if allowedNetworksStr != "" {
		allowedNetworks = strings.Split(allowedNetworksStr, ",")
		for i, network := range allowedNetworks {
			allowedNetworks[i] = strings.TrimSpace(network)
		}
	}

	// Parse TLS settings
	tlsEnable := false
	if tlsEnableStr != "" {
		switch strings.ToLower(tlsEnableStr) {
		case "true", "1", "yes", "on":
			tlsEnable = true
		case "false", "0", "no", "off":
			tlsEnable = false
		default:
			return nil, fmt.Errorf("invalid TLS_ENABLE value '%s': use true/false", tlsEnableStr)
		}
	}

	// Validate TLS configuration
	if tlsEnable {
		if tlsCertPath == "" {
			return nil, fmt.Errorf("TLS_CERT_PATH is required when TLS_ENABLE=true")
		}
		if tlsKeyPath == "" {
			return nil, fmt.Errorf("TLS_KEY_PATH is required when TLS_ENABLE=true")
		}

		// Check if certificate files exist
		if _, err := os.Stat(tlsCertPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("TLS certificate file not found: %s", tlsCertPath)
		}
		if _, err := os.Stat(tlsKeyPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("TLS key file not found: %s", tlsKeyPath)
		}
	}

	return &Config{
		TelegramBotToken: telegramBotToken,
		SlackBotToken:    slackBotToken,
		SMTPListenHost:   smtpHost,
		SMTPListenPort:   smtpPort,
		AllowedNetworks:  allowedNetworks,
		TLSEnable:        tlsEnable,
		TLSCertPath:      tlsCertPath,
		TLSKeyPath:       tlsKeyPath,
	}, nil
}

// Application represents the main application
type Application struct {
	Config         *Config
	TelegramClient *TelegramClient
	SlackClient    *SlackClient
	EmailProcessor *EmailProcessor
	SMTPServer     *SMTPServer
}

// loadTLSConfig loads TLS configuration if enabled
func loadTLSConfig(config *Config) (*tls.Config, error) {
	if !config.TLSEnable {
		return nil, nil
	}

	// Load certificate and key
	cert, err := tls.LoadX509KeyPair(config.TLSCertPath, config.TLSKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   "localhost",      // Can be overridden for production
		MinVersion:   tls.VersionTLS12, // Require at least TLS 1.2
	}

	log.Printf("TLS configuration loaded successfully")
	log.Printf("Certificate: %s", config.TLSCertPath)
	log.Printf("Private Key: %s", config.TLSKeyPath)

	return tlsConfig, nil
}

// validatePlatformTokens validates all configured platform tokens
func validatePlatformTokens(telegramClient *TelegramClient, slackClient *SlackClient) []error {
	var errors []error

	if telegramClient != nil {
		log.Println("Testing Telegram bot token...")
		if err := telegramClient.TestConnection(); err != nil {
			errors = append(errors, fmt.Errorf("telegram validation failed: %w", err))
		} else {
			log.Println("Telegram bot token validated successfully!")
		}
	}

	if slackClient != nil {
		log.Println("Testing Slack bot token...")
		if err := slackClient.TestConnection(); err != nil {
			errors = append(errors, fmt.Errorf("slack validation failed: %w", err))
		} else {
			log.Println("Slack bot token validated successfully!")
		}
	}

	return errors
}

// NewApplication creates a new application instance
func NewApplication(config *Config) (*Application, error) {
	// Load TLS configuration if enabled
	tlsConfig, err := loadTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("TLS configuration error: %w", err)
	}

	// Initialize platform clients
	var telegramClient *TelegramClient
	var slackClient *SlackClient

	if config.TelegramBotToken != "" {
		telegramClient = NewTelegramClient(config.TelegramBotToken)
	}

	if config.SlackBotToken != "" {
		slackClient = NewSlackClient(config.SlackBotToken)
	}

	// Initialize email processor with platform clients
	emailProcessor := NewEmailProcessor(telegramClient, slackClient)

	// Initialize SMTP server with TLS support
	smtpServer := NewSMTPServer(emailProcessor, config.SMTPListenHost, config.SMTPListenPort, config.AllowedNetworks, tlsConfig)

	return &Application{
		Config:         config,
		TelegramClient: telegramClient,
		SlackClient:    slackClient,
		EmailProcessor: emailProcessor,
		SMTPServer:     smtpServer,
	}, nil
}

// Start starts the application
func (app *Application) Start() error {
	log.Println("Starting email2dm - SMTP to Chat Platform Bridge...")

	// Test platform tokens
	log.Println("Validating platform tokens...")
	tokenErrors := validatePlatformTokens(app.TelegramClient, app.SlackClient)
	if len(tokenErrors) > 0 {
		for _, err := range tokenErrors {
			log.Printf("Warning: %v", err)
		}
		log.Println("Continuing anyway - some platforms may not work")
	}

	// Get bot info for debugging
	if app.TelegramClient != nil {
		if err := app.TelegramClient.GetBotInfo(); err != nil {
			log.Printf("Warning: Could not get Telegram bot info: %v", err)
		}
	}
	if app.SlackClient != nil {
		if err := app.SlackClient.GetBotInfo(); err != nil {
			log.Printf("Warning: Could not get Slack bot info: %v", err)
		}
	}

	// Start SMTP server
	log.Printf("Starting SMTP server on %s", app.SMTPServer.GetServerAddress())

	// Start server in a goroutine so we can handle shutdown signals
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.SMTPServer.Start()
	}()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("email2dm is running...")
	log.Println("Press Ctrl+C to stop")

	// Wait for either server error or shutdown signal
	select {
	case err := <-serverErr:
		return fmt.Errorf("SMTP server error: %w", err)
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		return app.Stop()
	}
}

// Stop stops the application gracefully
func (app *Application) Stop() error {
	log.Println("Shutting down SMTP to Telegram Bridge...")

	// Stop SMTP server
	if err := app.SMTPServer.Stop(); err != nil {
		log.Printf("Error stopping SMTP server: %v", err)
		return err
	}

	log.Println("SMTP to Telegram Bridge stopped successfully")
	return nil
}

// printUsage prints usage information
func printUsage() {
	usage := `email2dm - SMTP to Chat Platform Bridge
==========================================

This application creates an SMTP server that forwards emails to chat platforms.

Required Environment Variables:
  At least one platform token is required:
  TELEGRAM_BOT_TOKEN - Your Telegram bot token from @BotFather
  SLACK_BOT_TOKEN    - Your Slack bot token (xoxb-...)

Optional Environment Variables:
  SMTP_LISTEN_HOST   - IP address to bind SMTP server (default: 0.0.0.0)
  SMTP_LISTEN_PORT   - Port to bind SMTP server (default: 2525)
  ALLOWED_NETWORKS   - Comma-separated CIDR networks (e.g., '192.168.1.0/24,10.0.0.0/8')
  TLS_ENABLE         - Enable STARTTLS support (true/false, default: false)
  TLS_CERT_PATH      - Path to TLS certificate file (required if TLS_ENABLE=true)
  TLS_KEY_PATH       - Path to TLS private key file (required if TLS_ENABLE=true)

Email Address Format:
  Send emails to: <USER_ID>@<platform>
  
  Telegram Examples:
    123456789@telegram        # User ID 123456789
    -1001234567@telegram      # Group chat -1001234567
  
  Slack Examples:
    U1234567890@slack         # User ID U1234567890
    C1234567890@slack         # Channel ID C1234567890
    #general@slack            # Channel name #general
    @username@slack           # Username @username

Example Usage:
  # Basic setup (plain SMTP)
  export TELEGRAM_BOT_TOKEN='123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11'
  export SLACK_BOT_TOKEN='xoxb-1234567890-1234567890-abcdefghij'
  export SMTP_LISTEN_HOST='127.0.0.1'      # Optional: bind to localhost only
  export SMTP_LISTEN_PORT='2525'           # Optional: custom port
  export ALLOWED_NETWORKS='192.168.1.0/24' # Optional: restrict source IPs
  ./email2dm

  # With STARTTLS support
  export TELEGRAM_BOT_TOKEN='123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11'
  export SMTP_LISTEN_PORT='587'            # Standard submission port
  export TLS_ENABLE='true'
  export TLS_CERT_PATH='/path/to/server.crt'
  export TLS_KEY_PATH='/path/to/server.key'
  ./email2dm

Testing:
  # Plain SMTP
  swaks --to 123456789@telegram --from sender@company.com --server localhost:2525 --body 'Test message'
  swaks --to U1234567@slack --from sender@company.com --server localhost:2525 --body 'Test message'
  
  # With STARTTLS
  swaks --to 123456789@telegram --from sender@company.com --server localhost:587 --tls --body 'Test message'

TLS Support:
  STARTTLS allows both encrypted and unencrypted connections on the same port
  Clients can optionally upgrade to TLS encryption after connecting
  Self-signed certificates are supported for development/internal use

Use Cases:
  • Server monitoring alerts
  • Application deployment notifications  
  • Automated backup reports
  • Legacy hardware that only knows SMTP but wants to tell you how it's feeling

Logging:
  All email processing events are logged to syslog with format:
  src=<source_ip> from=<sender_email> platform=<platform> user_id=<chat_id> msg=<status>`

	fmt.Println(usage)
}

func main() {
	// Check if help was requested
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		printUsage()
		return // Exit immediately after printing help
	}

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Printf("Configuration error: %v", err)
		fmt.Println("")
		printUsage()
		os.Exit(1)
	}

	// Create and start application
	app, err := NewApplication(config)
	if err != nil {
		log.Fatalf("Application initialization error: %v", err)
	}

	if err := app.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
