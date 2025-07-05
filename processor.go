package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"mime"
	"net/mail"
	"strconv"
	"strings"
)

// EmailProcessor handles email parsing and processing
type EmailProcessor struct {
	TelegramClient *TelegramClient
	SyslogWriter   *syslog.Writer
}

// NewEmailProcessor creates a new email processor
func NewEmailProcessor(telegramClient *TelegramClient) *EmailProcessor {
	// Initialize syslog writer
	syslogWriter, err := syslog.New(syslog.LOG_INFO|syslog.LOG_MAIL, "smtp-telegram-bridge")
	if err != nil {
		log.Printf("Warning: failed to initialize syslog: %v", err)
		syslogWriter = nil
	}

	return &EmailProcessor{
		TelegramClient: telegramClient,
		SyslogWriter:   syslogWriter,
	}
}

// ProcessedEmail represents a processed email with extracted information
type ProcessedEmail struct {
	From    string
	To      string
	Subject string
	Date    string
	Body    string
}

// ProcessEmail processes raw email data and sends it to Telegram
func (ep *EmailProcessor) ProcessEmail(data []byte, from string, to []string, remoteAddr string) error {
	log.Printf("Processing email: %d bytes", len(data))

	// Extract Telegram ID from first TO address
	telegramID, err := ep.extractTelegramID(to)
	if err != nil {
		ep.logToSyslog(remoteAddr, from, "", fmt.Sprintf("Invalid destination: %v", err))
		return fmt.Errorf("invalid telegram destination: %w", err)
	}

	// Parse the email
	parsedEmail, err := ep.parseEmail(data)
	if err != nil {
		ep.logToSyslog(remoteAddr, from, telegramID, fmt.Sprintf("Parse error: %v", err))
		return fmt.Errorf("failed to parse email: %w", err)
	}

	// Log to syslog
	ep.logToSyslog(remoteAddr, from, telegramID, "Processing email")

	// Log the processed email info
	log.Printf("Processed email - From: %s, To Telegram: %s, Subject: %s",
		parsedEmail.From, telegramID, parsedEmail.Subject)

	// Format for Telegram
	telegramMessage := ep.formatForTelegram(parsedEmail)

	// Send to Telegram with dynamic chat ID
	if err := ep.TelegramClient.SendLongMessageToChat(telegramMessage, telegramID); err != nil {
		ep.logToSyslog(remoteAddr, from, telegramID, fmt.Sprintf("Telegram send failed: %v", err))
		return fmt.Errorf("failed to send to Telegram: %w", err)
	}

	ep.logToSyslog(remoteAddr, from, telegramID, "Email sent successfully")
	log.Println("Email successfully processed and sent to Telegram")
	return nil
}

// extractTelegramID extracts Telegram chat ID from the first email address
func (ep *EmailProcessor) extractTelegramID(toAddresses []string) (string, error) {
	if len(toAddresses) == 0 {
		return "", fmt.Errorf("no recipient addresses provided")
	}

	// Use only the first TO address
	firstAddress := toAddresses[0]

	// Parse email address to get local part
	addr, err := mail.ParseAddress(firstAddress)
	if err != nil {
		return "", fmt.Errorf("invalid email address format: %s", firstAddress)
	}

	// Split email to get local part (before @)
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid email address format: %s", addr.Address)
	}

	localPart := parts[0]

	// Validate that local part looks like a Telegram chat ID
	if err := ep.validateTelegramID(localPart); err != nil {
		return "", fmt.Errorf("invalid telegram ID '%s': %w", localPart, err)
	}

	return localPart, nil
}

// validateTelegramID validates if a string looks like a valid Telegram chat ID
func (ep *EmailProcessor) validateTelegramID(id string) error {
	if id == "" {
		return fmt.Errorf("empty ID")
	}

	// Parse as integer to validate format
	chatID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("not a valid integer: %w", err)
	}

	// Basic sanity checks for Telegram IDs
	if chatID == 0 {
		return fmt.Errorf("ID cannot be zero")
	}

	// Telegram user IDs are typically positive, group/channel IDs are negative
	// We accept both
	log.Printf("Validated Telegram ID: %d (type: %s)", chatID,
		map[bool]string{true: "user", false: "group/channel"}[chatID > 0])

	return nil
}

// logToSyslog logs email processing events to syslog
func (ep *EmailProcessor) logToSyslog(srcIP, fromAddr, telegramID, message string) {
	logMessage := fmt.Sprintf("src=%s from=%s telegram_id=%s msg=%s",
		srcIP, fromAddr, telegramID, message)

	if ep.SyslogWriter != nil {
		err := ep.SyslogWriter.Info(logMessage)
		if err != nil {
			log.Printf("Failed to write to syslog: %v", err)
		}
	} else {
		// Fallback to standard log if syslog unavailable
		log.Printf("SYSLOG: %s", logMessage)
	}
}
func (ep *EmailProcessor) parseEmail(data []byte) (*ProcessedEmail, error) {
	// Parse the email using Go's mail package
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to read email message: %w", err)
	}

	// Extract and decode headers
	from := ep.decodeHeader(msg.Header.Get("From"))
	to := ep.decodeHeader(msg.Header.Get("To"))
	subject := ep.decodeHeader(msg.Header.Get("Subject"))
	date := ep.formatDate(msg.Header.Get("Date"))

	// Clean email addresses
	from = ep.cleanEmailAddress(from)
	to = ep.cleanEmailAddress(to)

	// Extract body content
	body, err := ep.extractEmailBody(msg)
	if err != nil {
		log.Printf("Warning: failed to extract email body: %v", err)
		body = "[Unable to extract email body]"
	}

	return &ProcessedEmail{
		From:    from,
		To:      to,
		Subject: subject,
		Date:    date,
		Body:    body,
	}, nil
}

// decodeHeader decodes MIME-encoded email headers
func (ep *EmailProcessor) decodeHeader(header string) string {
	if header == "" {
		return ""
	}

	// Use Go's mime package to decode headers
	decoder := new(mime.WordDecoder)
	decoded, err := decoder.DecodeHeader(header)
	if err != nil {
		log.Printf("Warning: failed to decode header '%s': %v", header, err)
		return header // Return original if decoding fails
	}

	return strings.TrimSpace(decoded)
}

// cleanEmailAddress removes angle brackets and extracts clean email addresses
func (ep *EmailProcessor) cleanEmailAddress(addr string) string {
	if addr == "" {
		return ""
	}

	// Trim whitespace
	addr = strings.TrimSpace(addr)

	// Extract email from "Name <email@domain.com>" format
	if strings.Contains(addr, "<") && strings.Contains(addr, ">") {
		start := strings.Index(addr, "<")
		end := strings.Index(addr, ">")
		if start < end && start != -1 && end != -1 {
			return strings.TrimSpace(addr[start+1 : end])
		}
	}

	return addr
}

// formatDate formats the email date string for display
func (ep *EmailProcessor) formatDate(dateStr string) string {
	if dateStr == "" {
		return "Unknown"
	}

	// Parse the date using Go's mail package
	parsedTime, err := mail.ParseDate(dateStr)
	if err != nil {
		log.Printf("Warning: failed to parse date '%s': %v", dateStr, err)
		return dateStr // Return original if parsing fails
	}

	// Format in a readable way
	return parsedTime.UTC().Format("2006-01-02 15:04:05 UTC")
}

// extractEmailBody extracts the text content from an email
func (ep *EmailProcessor) extractEmailBody(msg *mail.Message) (string, error) {
	// Read the entire body
	bodyBytes, err := io.ReadAll(msg.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read message body: %w", err)
	}

	// Get content type from headers
	contentType := msg.Header.Get("Content-Type")
	contentTransferEncoding := msg.Header.Get("Content-Transfer-Encoding")

	log.Printf("Email content type: %s", contentType)
	log.Printf("Content transfer encoding: %s", contentTransferEncoding)

	// Handle different content types
	if strings.Contains(strings.ToLower(contentType), "multipart/") {
		return ep.extractFromMultipart(bodyBytes)
	}

	// Handle single-part messages
	bodyText := string(bodyBytes)

	// Clean up the body text
	bodyText = ep.cleanBodyText(bodyText)

	return bodyText, nil
}

// extractFromMultipart extracts text content from multipart messages
func (ep *EmailProcessor) extractFromMultipart(body []byte) (string, error) {
	bodyStr := string(body)
	var textContent strings.Builder

	// Split by common multipart boundaries
	// This is a simplified approach - in production you might want to use a proper MIME parser
	parts := strings.Split(bodyStr, "\n\n")

	inTextPart := false
	for _, part := range parts {
		lines := strings.Split(part, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)

			// Check if this is a text/plain part
			if strings.Contains(strings.ToLower(line), "content-type:") &&
				strings.Contains(strings.ToLower(line), "text/plain") {
				inTextPart = true
				continue
			}

			// Check if this is a boundary or other content type
			if strings.Contains(strings.ToLower(line), "content-type:") &&
				!strings.Contains(strings.ToLower(line), "text/plain") {
				inTextPart = false
				continue
			}

			// If we're in a text part and this isn't a header, add it to content
			if inTextPart && !strings.Contains(line, ":") && line != "" {
				textContent.WriteString(line)
				textContent.WriteString("\n")
			}
		}
	}

	result := textContent.String()
	if result == "" {
		// If no text/plain found, return the whole body (cleaned)
		result = ep.cleanBodyText(bodyStr)
	}

	return strings.TrimSpace(result), nil
}

// cleanBodyText cleans up body text by removing headers and formatting
func (ep *EmailProcessor) cleanBodyText(body string) string {
	lines := strings.Split(body, "\n")
	var cleanLines []string

	skipHeaders := true

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip email headers at the beginning
		if skipHeaders {
			// Headers typically contain colons and specific patterns
			if strings.Contains(line, ":") &&
				(strings.HasPrefix(strings.ToLower(line), "content-") ||
					strings.HasPrefix(strings.ToLower(line), "mime-") ||
					strings.HasPrefix(strings.ToLower(line), "return-") ||
					strings.HasPrefix(strings.ToLower(line), "received:") ||
					strings.HasPrefix(strings.ToLower(line), "message-id:")) {
				continue
			}

			// Empty line often marks end of headers
			if line == "" {
				skipHeaders = false
				continue
			}

			// If line doesn't look like a header, start including content
			if !strings.Contains(line, ":") && line != "" {
				skipHeaders = false
			}
		}

		if !skipHeaders {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// formatForTelegram formats the processed email for Telegram display
func (ep *EmailProcessor) formatForTelegram(email *ProcessedEmail) string {
	// Create a nicely formatted message for Telegram
	message := fmt.Sprintf(`📧 <b>New Email</b>

<b>From:</b> %s
<b>To:</b> %s
<b>Subject:</b> %s
<b>Date:</b> %s

<b>Message:</b>
%s`,
		ep.escapeHTML(email.From),
		ep.escapeHTML(email.To),
		ep.escapeHTML(email.Subject),
		ep.escapeHTML(email.Date),
		ep.escapeHTML(email.Body))

	return message
}

// escapeHTML escapes HTML special characters for Telegram
func (ep *EmailProcessor) escapeHTML(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
	)
	return replacer.Replace(text)
}

// GetProcessorStats returns basic statistics about processed emails
func (ep *EmailProcessor) GetProcessorStats() map[string]interface{} {
	// This could be expanded to track actual statistics
	return map[string]interface{}{
		"status":             "active",
		"telegram_connected": ep.TelegramClient != nil,
	}
}
