#!/usr/bin/env bash

# SMTP to Telegram Bridge Launch Script
# This script launches the bridge with basic configuration

set -e  # Exit on any error

echo "🚀 Starting SMTP to Telegram Bridge..."
echo "Configuration:"
echo "  SMTP Host: 0.0.0.0"
echo "  SMTP Port: 2525"
echo "  TLS: Disabled"
echo ""

# Set environment variables
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
export SMTP_LISTEN_HOST="0.0.0.0"
export SMTP_LISTEN_PORT="2525"

# Optional: Uncomment these for additional configuration
# export TELEGRAM_CHAT_ID="123456789"                    # Default chat ID for testing
# export ALLOWED_NETWORKS="192.168.1.0/24,10.0.0.0/8"  # Network ACL
# export TLS_ENABLE="false"                              # Explicitly disable TLS

echo "⚠️  IMPORTANT: Update TELEGRAM_BOT_TOKEN with your actual bot token!"
echo "   Get your token from @BotFather on Telegram"
echo ""
echo "📧 Send emails to: <TELEGRAM_ID>@anydomain.com"
echo "   Examples:"
echo "     123456789@notifications.company.com     # User ID 123456789"
echo "     -1001234567@alerts.company.com          # Group chat -1001234567"
echo ""
echo "🧪 Test with: swaks --to 123456789@example.com --from test@company.com --server localhost:2525 --body 'Test message'"
echo ""

# Check if binary exists
if [ ! -f "./email2dm" ]; then
    echo "❌ Binary not found! Please build first with: go build"
    exit 1
fi

# Launch the application
echo "📡 Starting SMTP server on 0.0.0.0:2525..."
echo "Press Ctrl+C to stop"
echo ""

./email2dm 
