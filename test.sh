#!/bin/bash

# email2dm - SMTP to Chat Platform Bridge Launch Script
# This script launches the bridge with basic configuration

set -e  # Exit on any error

echo "🚀 Starting email2dm - SMTP to Chat Platform Bridge..."
echo "Configuration:"
echo "  SMTP Host: 0.0.0.0"
echo "  SMTP Port: 2525"
echo "  TLS: Disabled"
echo ""

# Set environment variables
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
export SLACK_BOT_TOKEN="xoxb-1234567890-1234567890-abcdefghij"
export SMTP_LISTEN_HOST="0.0.0.0"
export SMTP_LISTEN_PORT="2525"

# Optional: Uncomment these for additional configuration
# export ALLOWED_NETWORKS="192.168.1.0/24,10.0.0.0/8"  # Network ACL
# export TLS_ENABLE="true"                              # Enable STARTTLS
# export TLS_CERT_PATH="/path/to/server.crt"           # TLS Certificate
# export TLS_KEY_PATH="/path/to/server.key"            # TLS Private Key

echo "⚠️  IMPORTANT: Update bot tokens with your actual tokens!"
echo "   Telegram: Get your token from @BotFather on Telegram"
echo "   Slack: Get your token from https://api.slack.com/apps"
echo ""
echo "📧 Send emails to: <USER_ID>@<platform>"
echo ""
echo "   Telegram Examples:"
echo "     123456789@telegram        # User ID 123456789"
echo "     -1001234567@telegram      # Group chat -1001234567"
echo ""
echo "   Slack Examples:"
echo "     U1234567890@slack         # User ID U1234567890"
echo "     C1234567890@slack         # Channel ID C1234567890"
echo "     #general@slack            # Channel name #general"
echo "     @username@slack           # Username @username"
echo ""
echo "🧪 Test commands:"
echo "   Telegram: swaks --to 123456789@telegram --from test@company.com --server localhost:2525 --body 'Test message'"
echo "   Slack:    swaks --to '#general@slack' --from test@company.com --server localhost:2525 --body 'Test message'"
echo ""

# Check if binary exists
if [ ! -f "./email2dm" ]; then
    echo "❌ Binary not found! Please build first with:"
    echo "   go build -o email2dm"
    exit 1
fi

# Launch the application
echo "📡 Starting SMTP server on 0.0.0.0:2525..."
echo "Configured platforms: Telegram, Slack"
echo "Press Ctrl+C to stop"
echo ""

./email2dm