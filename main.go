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
	SMTPListenHost   string
	SMTPListenPort   int
	AllowedNetworks  []string
	TLSEnable        bool
	TLSCertPath      string
	TLSKeyPath       string
}

// loadConfig loads configuration from environment variables
func loadConfig() (*Config, error) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	smtpHost := os.Getenv("SMTP_LISTEN_HOST")
	smtpPortStr := os.Getenv("SMTP_LISTEN_PORT")
	allowedNetworksStr := os.Getenv("ALLOWED_NETWORKS")
	tlsEnableStr := os.Getenv("TLS_ENABLE")
	tlsCertPath := os.Getenv("TLS_CERT_PATH")
	tlsKeyPath := os.Getenv("TLS_KEY_PATH")

	if botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required")
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
		TelegramBotToken: botToken,
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

// NewApplication creates a new application instance
func NewApplication(config *Config) (*Application, error) {
	// Load TLS configuration if enabled
	tlsConfig, err := loadTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("TLS configuration error: %w", err)
	}

	// Initialize Telegram client
	telegramClient := NewTelegramClient(config.TelegramBotToken)

	// Initialize email processor
	emailProcessor := NewEmailProcessor(telegramClient)

	// Initialize SMTP server with TLS support
	smtpServer := NewSMTPServer(emailProcessor, config.SMTPListenHost, config.SMTPListenPort, config.AllowedNetworks, tlsConfig)

	return &Application{
		Config:         config,
		TelegramClient: telegramClient,
		EmailProcessor: emailProcessor,
		SMTPServer:     smtpServer,
	}, nil
}

// Start starts the application
func (app *Application) Start() error {
	log.Println("Starting SMTP to Telegram Bridge...")

	// Test Telegram bot token and API access
	log.Println("Testing Telegram bot token...")
	if err := app.TelegramClient.TestConnection(); err != nil {
		log.Printf("Warning: Telegram bot validation failed: %v", err)
		log.Println("Continuing anyway - check your bot token")
	} else {
		log.Println("Telegram bot token validated successfully!")
	}

	// Get bot info for debugging
	if err := app.TelegramClient.GetBotInfo(); err != nil {
		log.Printf("Warning: Could not get bot info: %v", err)
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

	log.Println("SMTP to Telegram Bridge is running...")
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
	fmt.Println("SMTP to Telegram Bridge")
	fmt.Println("=======================")
	fmt.Println("")
	fmt.Println("This application creates an SMTP server that forwards emails to Telegram.")
	fmt.Println("")
	fmt.Println("Required environment variables:")
	fmt.Println("  TELEGRAM_BOT_TOKEN - Your Telegram bot token from @BotFather")
	fmt.Println("")
	fmt.Println("Optional environment variables:")
	fmt.Println("  SMTP_LISTEN_HOST   - IP address to bind SMTP server (default: 0.0.0.0)")
	fmt.Println("  SMTP_LISTEN_PORT   - Port to bind SMTP server (default: 2525)")
	fmt.Println("  ALLOWED_NETWORKS   - Comma-separated CIDR networks (e.g., '192.168.1.0/24,10.0.0.0/8')")
	fmt.Println("  TLS_ENABLE         - Enable STARTTLS support (true/false, default: false)")
	fmt.Println("  TLS_CERT_PATH      - Path to TLS certificate file (required if TLS_ENABLE=true)")
	fmt.Println("  TLS_KEY_PATH       - Path to TLS private key file (required if TLS_ENABLE=true)")
	fmt.Println("")
	fmt.Println("Email Address Format:")
	fmt.Println("  Send emails to: <TELEGRAM_ID>@<any-domain>")
	fmt.Println("  Examples:")
	fmt.Println("    123456789@notifications.company.com     # User ID 123456789")
	fmt.Println("    -1001234567@alerts.company.com          # Group chat -1001234567")
	fmt.Println("")
	fmt.Println("Example usage:")
	fmt.Println("  # Basic setup (plain SMTP)")
	fmt.Println("  export TELEGRAM_BOT_TOKEN='123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11'")
	fmt.Println("  export SMTP_LISTEN_HOST='127.0.0.1'      # Optional: bind to localhost only")
	fmt.Println("  export SMTP_LISTEN_PORT='2525'           # Optional: custom port")
	fmt.Println("  export ALLOWED_NETWORKS='192.168.1.0/24' # Optional: restrict source IPs")
	fmt.Println("  ./smtp-telegram-bridge")
	fmt.Println("")
	fmt.Println("  # With STARTTLS support")
	fmt.Println("  export TELEGRAM_BOT_TOKEN='123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11'")
	fmt.Println("  export SMTP_LISTEN_PORT='587'            # Standard submission port")
	fmt.Println("  export TLS_ENABLE='true'")
	fmt.Println("  export TLS_CERT_PATH='/path/to/server.crt'")
	fmt.Println("  export TLS_KEY_PATH='/path/to/server.key'")
	fmt.Println("  ./smtp-telegram-bridge")
	fmt.Println("")
	fmt.Println("The SMTP server will start on the configured host and port")
	fmt.Println("You can test it with tools like swaks:")
	fmt.Println("  # Plain SMTP")
	fmt.Println("  swaks --to 123456789@example.com --from sender@company.com --server localhost:2525 --body 'Test message'")
	fmt.Println("  # With STARTTLS")
	fmt.Println("  swaks --to 123456789@example.com --from sender@company.com --server localhost:587 --tls --body 'Test message'")
	fmt.Println("")
	fmt.Println("TLS Support:")
	fmt.Println("  STARTTLS allows both encrypted and unencrypted connections on the same port")
	fmt.Println("  Clients can optionally upgrade to TLS encryption after connecting")
	fmt.Println("  Self-signed certificates are supported for development/internal use")
	fmt.Println("")
	fmt.Println("Logging:")
	fmt.Println("  All email processing events are logged to syslog with format:")
	fmt.Println("  src=<source_ip> from=<sender_email> telegram_id=<chat_id> msg=<status>")
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
