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
	SlackClient    *SlackClient
	SyslogWriter   *syslog.Writer
}

// NewEmailProcessor creates a new email processor
func NewEmailProcessor(telegramClient *TelegramClient, slackClient *SlackClient) *EmailProcessor {
	// Initialize syslog writer
	syslogWriter, err := syslog.New(syslog.LOG_INFO|syslog.LOG_MAIL, "email2dm")
	if err != nil {
		log.Printf("Warning: failed to initialize syslog: %v", err)
		syslogWriter = nil
	}

	return &EmailProcessor{
		TelegramClient: telegramClient,
		SlackClient:    slackClient,
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

// ProcessEmail processes raw email data and sends it to the appropriate platform
func (ep *EmailProcessor) ProcessEmail(data []byte, from string, to []string, remoteAddr string) error {
	log.Printf("Processing email: %d bytes", len(data))

	// Extract platform and ID from first TO address
	platform, userID, err := ep.extractPlatformAndID(to)
	if err != nil {
		ep.logToSyslog(remoteAddr, from, "", "", fmt.Sprintf("Invalid destination: %v", err))
		return fmt.Errorf("invalid destination: %w", err)
	}

	// Parse the email
	parsedEmail, err := ep.parseEmail(data)
	if err != nil {
		ep.logToSyslog(remoteAddr, from, platform, userID, fmt.Sprintf("Parse error: %v", err))
		return fmt.Errorf("failed to parse email: %w", err)
	}

	// Log to syslog
	ep.logToSyslog(remoteAddr, from, platform, userID, "Processing email")

	// Log the processed email info
	log.Printf("Processed email - From: %s, To %s: %s, Subject: %s",
		parsedEmail.From, platform, userID, parsedEmail.Subject)

	// Format message for the specific platform
	message := ep.formatMessageForPlatform(parsedEmail, platform)

	// Send to the appropriate platform
	if err := ep.sendToPlatform(message, platform, userID); err != nil {
		ep.logToSyslog(remoteAddr, from, platform, userID, fmt.Sprintf("Send failed: %v", err))
		return fmt.Errorf("failed to send to %s: %w", platform, err)
	}

	ep.logToSyslog(remoteAddr, from, platform, userID, "Email sent successfully")
	log.Println("Email successfully processed and sent")
	return nil
}

// extractPlatformAndID extracts platform and user ID from the first email address
func (ep *EmailProcessor) extractPlatformAndID(toAddresses []string) (platform, userID string, err error) {
	if len(toAddresses) == 0 {
		return "", "", fmt.Errorf("no recipient addresses provided")
	}

	// Use only the first TO address
	firstAddress := toAddresses[0]

	// Parse email address to get local and domain parts
	addr, err := mail.ParseAddress(firstAddress)
	if err != nil {
		return "", "", fmt.Errorf("invalid email address format: %s", firstAddress)
	}

	// Split email to get local part (before @) and domain (after @)
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid email address format: %s", addr.Address)
	}

	localPart := parts[0]
	domainPart := strings.ToLower(parts[1])

	// Determine platform from domain
	switch domainPart {
	case "telegram":
		platform = "telegram"
	case "slack":
		platform = "slack"
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", domainPart)
	}

	// Validate the ID for the specific platform
	if err := ep.validateIDForPlatform(localPart, platform); err != nil {
		return "", "", fmt.Errorf("invalid %s ID '%s': %w", platform, localPart, err)
	}

	return platform, localPart, nil
}

// validateIDForPlatform validates if a string looks like a valid ID for the specified platform
func (ep *EmailProcessor) validateIDForPlatform(id, platform string) error {
	if id == "" {
		return fmt.Errorf("empty ID")
	}

	switch platform {
	case "telegram":
		return ep.validateTelegramID(id)
	case "slack":
		return ep.validateSlackID(id)
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}
}

// validateTelegramID validates if a string looks like a valid Telegram chat ID
func (ep *EmailProcessor) validateTelegramID(id string) error {
	// Handle group prefix notation: g123456 -> -123456
	if strings.HasPrefix(id, "g") && len(id) > 1 {
		// Remove 'g' prefix and validate the rest as a number
		numPart := id[1:]
		if _, err := strconv.ParseInt(numPart, 10, 64); err != nil {
			return fmt.Errorf("invalid group ID format 'g%s': %w", numPart, err)
		}
		log.Printf("Validated Telegram group ID: g%s (will convert to -%s)", numPart, numPart)
		return nil
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
	log.Printf("Validated Telegram ID: %d (type: %s)", chatID,
		map[bool]string{true: "user", false: "group/channel"}[chatID > 0])

	return nil
}

// validateSlackID validates if a string looks like a valid Slack ID
func (ep *EmailProcessor) validateSlackID(id string) error {
	// Slack IDs can be:
	// - User IDs: U1234567890 (start with U)
	// - Channel IDs: C1234567890 (start with C)
	// - Channel names: #general (start with #)
	// - Usernames: username (plain username without @)

	if strings.HasPrefix(id, "U") && len(id) >= 9 {
		// User ID format
		log.Printf("Validated Slack user ID: %s", id)
		return nil
	}

	if strings.HasPrefix(id, "C") && len(id) >= 9 {
		// Channel ID format
		log.Printf("Validated Slack channel ID: %s", id)
		return nil
	}

	if strings.HasPrefix(id, "#") && len(id) > 1 {
		// Channel name format
		log.Printf("Validated Slack channel name: %s", id)
		return nil
	}

	// Plain username format (no @ prefix needed) - will be resolved to User ID later
	if len(id) > 0 && !strings.Contains(id, "#") && !strings.Contains(id, "@") {
		log.Printf("Validated Slack username: %s (will resolve to User ID)", id)
		return nil
	}

	return fmt.Errorf("invalid Slack ID format (expected U1234567890, C1234567890, #channel, or username)")
}

// sendToPlatform routes the message to the appropriate platform client
func (ep *EmailProcessor) sendToPlatform(message, platform, userID string) error {
	switch platform {
	case "telegram":
		if ep.TelegramClient == nil {
			return fmt.Errorf("telegram client not configured")
		}

		// Convert group prefix notation: g123456 -> -123456
		telegramID := userID
		if strings.HasPrefix(userID, "g") && len(userID) > 1 {
			telegramID = "-" + userID[1:]
			log.Printf("Converted group ID: %s -> %s", userID, telegramID)
		}

		return ep.TelegramClient.SendLongMessageToChat(message, telegramID)

	case "slack":
		if ep.SlackClient == nil {
			return fmt.Errorf("slack client not configured")
		}

		// Resolve username to User ID if needed
		resolvedID := userID
		if !strings.HasPrefix(userID, "U") && !strings.HasPrefix(userID, "C") && !strings.HasPrefix(userID, "#") {
			// This looks like a username, try to resolve it
			log.Printf("Attempting to resolve Slack username '%s' to User ID", userID)
			resolvedUserID, err := ep.SlackClient.ResolveUserID(userID)
			if err != nil {
				return fmt.Errorf("failed to resolve username '%s': %w", userID, err)
			}
			resolvedID = resolvedUserID
			log.Printf("Resolved username '%s' to User ID '%s'", userID, resolvedID)
		}

		return ep.SlackClient.SendLongMessageToChannel(message, resolvedID)

	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}
}

// formatMessageForPlatform formats the processed email for the specific platform
func (ep *EmailProcessor) formatMessageForPlatform(email *ProcessedEmail, platform string) string {
	switch platform {
	case "telegram":
		return ep.formatForTelegram(email)
	case "slack":
		return ep.formatForSlack(email)
	default:
		// Fallback to plain text
		return fmt.Sprintf("New Email\nFrom: %s\nTo: %s\nSubject: %s\nDate: %s\n\nMessage:\n%s",
			email.From, email.To, email.Subject, email.Date, email.Body)
	}
}

// logToSyslog logs email processing events to syslog
func (ep *EmailProcessor) logToSyslog(srcIP, fromAddr, platform, userID, message string) {
	logMessage := fmt.Sprintf("src=%s from=%s platform=%s user_id=%s msg=%s",
		srcIP, fromAddr, platform, userID, message)

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

// parseEmail parses raw email data into a ProcessedEmail struct
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
	message := fmt.Sprintf("📧 <b>New Email</b>\n\n<b>From:</b> %s\n<b>To:</b> %s\n<b>Subject:</b> %s\n<b>Date:</b> %s\n\n<b>Message:</b>\n%s",
		ep.escapeHTML(email.From),
		ep.escapeHTML(email.To),
		ep.escapeHTML(email.Subject),
		ep.escapeHTML(email.Date),
		ep.escapeHTML(email.Body))

	return message
}

// formatForSlack formats the processed email for Slack display (using Slack markdown)
func (ep *EmailProcessor) formatForSlack(email *ProcessedEmail) string {
	// Create a nicely formatted message for Slack using markdown
	message := fmt.Sprintf(":email: *New Email*\n\n*From:* %s\n*To:* %s\n*Subject:* %s\n*Date:* %s\n\n*Message:*\n```\n%s\n```",
		email.From,
		email.To,
		email.Subject,
		email.Date,
		email.Body)

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
		"slack_connected":    ep.SlackClient != nil,
	}
}
