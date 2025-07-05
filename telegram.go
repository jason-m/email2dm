package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Telegram Configuration
const (
	TelegramAPIURL     = "https://api.telegram.org/bot%s/sendMessage"
	MaxMessageLength   = 4096                   // Telegram's message limit
	MessageSendDelay   = 500 * time.Millisecond // Delay between message chunks
	HTTPRequestTimeout = 10 * time.Second
)

// TelegramMessage represents a message payload for Telegram API
type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// TelegramClient handles all Telegram API interactions
type TelegramClient struct {
	BotToken   string
	ChatID     string
	APIUrl     string
	HTTPClient *http.Client
}

// NewTelegramClient creates a new Telegram client
func NewTelegramClient(botToken, chatID string) *TelegramClient {
	return &TelegramClient{
		BotToken: botToken,
		ChatID:   chatID,
		APIUrl:   fmt.Sprintf(TelegramAPIURL, botToken),
		HTTPClient: &http.Client{
			Timeout: HTTPRequestTimeout,
		},
	}
}

// SendLongMessageToChat handles long messages by splitting them into chunks for a specific chat
func (tc *TelegramClient) SendLongMessageToChat(text, chatID string) error {
	if len(text) <= MaxMessageLength {
		return tc.SendMessageToChat(text, chatID)
	}

	log.Printf("Message too long (%d chars), splitting into chunks for chat %s", len(text), chatID)
	chunks := tc.splitMessage(text)

	for i, chunk := range chunks {
		// Add part number for continuation messages
		if i > 0 {
			chunk = fmt.Sprintf("[Part %d]\n%s", i+1, chunk)
		}

		if err := tc.SendMessageToChat(chunk, chatID); err != nil {
			return fmt.Errorf("failed to send chunk %d/%d to chat %s: %w", i+1, len(chunks), chatID, err)
		}

		// Add delay between messages to avoid rate limiting
		if i < len(chunks)-1 {
			log.Printf("Sent chunk %d/%d to chat %s, waiting before next...", i+1, len(chunks), chatID)
			time.Sleep(MessageSendDelay)
		}
	}

	log.Printf("Successfully sent all %d message chunks to chat %s", len(chunks), chatID)
	return nil
}

// SendMessageToChat sends a message to a specific chat ID
func (tc *TelegramClient) SendMessageToChat(text, chatID string) error {
	return tc.SendMessageToChatWithParseMode(text, chatID, "HTML")
}

// SendMessageToChatWithParseMode sends a message to a specific chat with specified parse mode
func (tc *TelegramClient) SendMessageToChatWithParseMode(text, chatID, parseMode string) error {
	message := TelegramMessage{
		ChatID:    chatID,
		Text:      text,
		ParseMode: parseMode,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	log.Printf("Sending message to Telegram chat %s (length: %d)", chatID, len(text))

	resp, err := tc.HTTPClient.Post(tc.APIUrl, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %d - %s", resp.StatusCode, string(body))
	}

	log.Printf("Message sent successfully to Telegram chat %s", chatID)
	return nil
}

// SendMessage sends a single message to the default chat ID (for backward compatibility)
func (tc *TelegramClient) SendMessage(text string) error {
	return tc.SendMessageToChat(text, tc.ChatID)
}

// SendLongMessage handles long messages by splitting them into chunks (for backward compatibility)
func (tc *TelegramClient) SendLongMessage(text string) error {
	return tc.SendLongMessageToChat(text, tc.ChatID)
}

// SendMessageWithParseMode sends a message with specified parse mode (for backward compatibility)
func (tc *TelegramClient) SendMessageWithParseMode(text, parseMode string) error {
	return tc.SendMessageToChatWithParseMode(text, tc.ChatID, parseMode)
}

// splitMessage splits a message into chunks that fit within Telegram's limits
func (tc *TelegramClient) splitMessage(text string) []string {
	var chunks []string
	lines := strings.Split(text, "\n")
	var currentChunk strings.Builder

	for _, line := range lines {
		// Check if adding this line would exceed the limit
		if currentChunk.Len()+len(line)+1 > MaxMessageLength {
			// Save current chunk if it has content
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				currentChunk.Reset()
			}

			// Handle very long lines by wrapping them
			if len(line) > MaxMessageLength-100 {
				wrappedLines := tc.wrapLongLine(line, MaxMessageLength-100)
				for j, wrappedLine := range wrappedLines {
					if j == 0 && currentChunk.Len() == 0 {
						// First wrapped line can go in current chunk
						currentChunk.WriteString(wrappedLine)
					} else {
						// Additional wrapped lines become separate chunks
						if currentChunk.Len() > 0 {
							chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
							currentChunk.Reset()
						}
						currentChunk.WriteString(wrappedLine)
					}
				}
			} else {
				currentChunk.WriteString(line)
			}
		} else {
			// Add line to current chunk
			if currentChunk.Len() > 0 {
				currentChunk.WriteString("\n")
			}
			currentChunk.WriteString(line)
		}
	}

	// Don't forget the last chunk
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

// wrapLongLine wraps a single long line into multiple lines
func (tc *TelegramClient) wrapLongLine(line string, maxLength int) []string {
	var wrapped []string

	for len(line) > maxLength {
		// Try to break at a space near the limit
		breakPoint := maxLength

		// Look for a space within the last 50 characters
		for i := maxLength - 1; i >= maxLength-50 && i >= 0; i-- {
			if line[i] == ' ' {
				breakPoint = i
				break
			}
		}

		// Extract the chunk
		chunk := line[:breakPoint]
		wrapped = append(wrapped, chunk)

		// Update remaining line
		line = line[breakPoint:]
		if len(line) > 0 && line[0] == ' ' {
			line = line[1:] // Remove leading space
		}
	}

	// Add remaining text
	if len(line) > 0 {
		wrapped = append(wrapped, line)
	}

	return wrapped
}

// SendPlainMessage sends a message without HTML formatting
func (tc *TelegramClient) SendPlainMessage(text string) error {
	return tc.SendMessageToChatWithParseMode(text, tc.ChatID, "")
}

// TestConnection tests the connection to Telegram by sending a test message
func (tc *TelegramClient) TestConnection() error {
	testMessage := "🔧 SMTP to Telegram Bridge - Connection Test"
	return tc.SendMessage(testMessage)
}

// GetBotInfo retrieves information about the bot (useful for debugging)
func (tc *TelegramClient) GetBotInfo() error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", tc.BotToken)

	resp, err := tc.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("Bot info: %s", string(body))
	return nil
}
