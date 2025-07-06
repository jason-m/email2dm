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

// Slack Configuration
const (
	SlackAPIURL             = "https://slack.com/api"
	SlackMaxMessageLength   = 40000                   // Slack's message limit (much higher than Telegram)
	SlackMessageSendDelay   = 1000 * time.Millisecond // Delay between message chunks
	SlackHTTPRequestTimeout = 10 * time.Second
)

// SlackMessage represents a message payload for Slack API
type SlackMessage struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
	AsUser  bool   `json:"as_user"`
}

// SlackClient handles all Slack API interactions
type SlackClient struct {
	BotToken   string
	HTTPClient *http.Client
}

// NewSlackClient creates a new Slack client
func NewSlackClient(botToken string) *SlackClient {
	return &SlackClient{
		BotToken: botToken,
		HTTPClient: &http.Client{
			Timeout: SlackHTTPRequestTimeout,
		},
	}
}

// SendLongMessageToChannel handles long messages by splitting them into chunks for a specific channel
func (sc *SlackClient) SendLongMessageToChannel(text, channelID string) error {
	if len(text) <= SlackMaxMessageLength {
		return sc.SendMessageToChannel(text, channelID)
	}

	log.Printf("Message too long (%d chars), splitting into chunks for Slack channel %s", len(text), channelID)
	chunks := sc.splitMessage(text)

	for i, chunk := range chunks {
		// Add part number for continuation messages
		if i > 0 {
			chunk = fmt.Sprintf("*[Part %d]*\n%s", i+1, chunk)
		}

		if err := sc.SendMessageToChannel(chunk, channelID); err != nil {
			return fmt.Errorf("failed to send chunk %d/%d to Slack channel %s: %w", i+1, len(chunks), channelID, err)
		}

		// Add delay between messages to avoid rate limiting
		if i < len(chunks)-1 {
			log.Printf("Sent chunk %d/%d to Slack channel %s, waiting before next...", i+1, len(chunks), channelID)
			time.Sleep(SlackMessageSendDelay)
		}
	}

	log.Printf("Successfully sent all %d message chunks to Slack channel %s", len(chunks), channelID)
	return nil
}

// SendMessageToChannel sends a message to a specific Slack channel
func (sc *SlackClient) SendMessageToChannel(text, channelID string) error {
	url := fmt.Sprintf("%s/chat.postMessage", SlackAPIURL)

	message := SlackMessage{
		Channel: channelID,
		Text:    text,
		AsUser:  true,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	log.Printf("Sending message to Slack channel %s (length: %d)", channelID, len(text))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", sc.BotToken))

	resp, err := sc.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse response to check for Slack-specific errors
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if ok, exists := response["ok"].(bool); !exists || !ok {
		errorMsg := "unknown error"
		if errField, exists := response["error"].(string); exists {
			errorMsg = errField
		}
		return fmt.Errorf("slack API error: %s", errorMsg)
	}

	log.Printf("Message sent successfully to Slack channel %s", channelID)
	return nil
}

// splitMessage splits a message into chunks that fit within Slack's limits
func (sc *SlackClient) splitMessage(text string) []string {
	var chunks []string
	lines := strings.Split(text, "\n")
	var currentChunk strings.Builder

	for _, line := range lines {
		// Check if adding this line would exceed the limit
		if currentChunk.Len()+len(line)+1 > SlackMaxMessageLength {
			// Save current chunk if it has content
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				currentChunk.Reset()
			}

			// Handle very long lines by wrapping them
			if len(line) > SlackMaxMessageLength-100 {
				wrappedLines := sc.wrapLongLine(line, SlackMaxMessageLength-100)
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
func (sc *SlackClient) wrapLongLine(line string, maxLength int) []string {
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

// TestConnection validates the bot token by checking auth test
func (sc *SlackClient) TestConnection() error {
	return sc.GetBotInfo()
}

// GetBotInfo retrieves information about the bot (useful for debugging)
func (sc *SlackClient) GetBotInfo() error {
	url := fmt.Sprintf("%s/auth.test", SlackAPIURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", sc.BotToken))

	resp, err := sc.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack API error: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response to check for Slack-specific errors
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if ok, exists := response["ok"].(bool); !exists || !ok {
		errorMsg := "unknown error"
		if errField, exists := response["error"].(string); exists {
			errorMsg = errField
		}
		return fmt.Errorf("slack auth test failed: %s", errorMsg)
	}

	log.Printf("Slack bot info: %s", string(body))
	return nil
}
