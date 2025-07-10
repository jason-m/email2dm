#!/usr/bin/env bash

# email2dm Setup Script
# Generates self-signed certificates, configures environment, and starts the bridge

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}=====================================${NC}"
echo -e "${BLUE}  email2dm Setup & Launch Script${NC}"
echo -e "${BLUE}=====================================${NC}"
echo ""

# Check if binary exists
if [ ! -f "./email2dm" ]; then
    echo -e "${RED}❌ email2dm binary not found!${NC}"
    echo "Please build it first with: go build -o email2dm"
    exit 1
fi

# Function to prompt for input with default
prompt_with_default() {
    local prompt="$1"
    local default="$2"
    local var_name="$3"

    if [ -n "$default" ]; then
        read -p "$prompt [$default]: " input
        if [ -z "$input" ]; then
            input="$default"
        fi
    else
        read -p "$prompt: " input
    fi

    eval "$var_name='$input'"
}

# Generate self-signed certificates
echo -e "${CYAN}🔐 Generating self-signed TLS certificates...${NC}"

if [ -f "server.crt" ] && [ -f "server.key" ]; then
    echo -e "${YELLOW}⚠️  Certificates already exist. Regenerate? (y/N)${NC}"
    read -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -f server.crt server.key
    fi
fi

if [ ! -f "server.crt" ] || [ ! -f "server.key" ]; then
    echo "Generating new certificates..."

    # Generate private key
    openssl genrsa -out server.key 2048 2>/dev/null

    # Generate certificate with multiple hostnames
    openssl req -new -x509 -key server.key -out server.crt -days 365 \
        -subj "/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,DNS:mail.local,IP:127.0.0.1,IP:0.0.0.0" \
        2>/dev/null

    echo -e "${GREEN}✅ Certificates generated successfully!${NC}"
    echo "   - server.crt (certificate)"
    echo "   - server.key (private key)"
else
    echo -e "${GREEN}✅ Using existing certificates${NC}"
fi

echo ""

# Prompt for API tokens
echo -e "${CYAN}🔑 Platform API Configuration${NC}"
echo "Enter your platform bot tokens (at least one is required):"
echo ""

# Telegram token
echo -e "${YELLOW}Telegram Bot Token:${NC}"
echo "Get from: @BotFather on Telegram"
prompt_with_default "TELEGRAM_BOT_TOKEN" "" "TELEGRAM_TOKEN"

echo ""

# Slack token
echo -e "${YELLOW}Slack Bot Token:${NC}"
echo "Get from: https://api.slack.com/apps (xoxb-...)"
echo "Required scopes: chat:write, chat:write.public, users:read, im:write"
prompt_with_default "SLACK_BOT_TOKEN" "" "SLACK_TOKEN"

echo ""

# Validate at least one token
if [ -z "$TELEGRAM_TOKEN" ] && [ -z "$SLACK_TOKEN" ]; then
    echo -e "${RED}❌ Error: At least one platform token is required!${NC}"
    exit 1
fi

# Prompt for configuration options
echo -e "${CYAN}⚙️  Server Configuration${NC}"

prompt_with_default "SMTP Listen Host" "127.0.0.1" "SMTP_HOST"
prompt_with_default "SMTP Port (plain)" "2525" "SMTP_PORT"
prompt_with_default "SMTP Port (with TLS)" "587" "TLS_PORT"

echo ""

# Set up environment variables
echo -e "${CYAN}🔧 Setting up environment...${NC}"

export SMTP_LISTEN_HOST="$SMTP_HOST"
export SMTP_LISTEN_PORT="$SMTP_PORT"
export ALLOWED_NETWORKS="127.0.0.1/32"
export TLS_ENABLE="true"
export TLS_CERT_PATH="./server.crt"
export TLS_KEY_PATH="./server.key"

if [ -n "$TELEGRAM_TOKEN" ]; then
    export TELEGRAM_BOT_TOKEN="$TELEGRAM_TOKEN"
    echo -e "${GREEN}✅ Telegram configured${NC}"
fi

if [ -n "$SLACK_TOKEN" ]; then
    export SLACK_BOT_TOKEN="$SLACK_TOKEN"
    echo -e "${GREEN}✅ Slack configured${NC}"
fi

echo -e "${GREEN}✅ ACL configured: 127.0.0.1/32 (localhost only)${NC}"
echo -e "${GREEN}✅ TLS enabled with self-signed certificates${NC}"

echo ""

# Display test commands
echo -e "${BLUE}=====================================${NC}"
echo -e "${BLUE}     Testing Commands${NC}"
echo -e "${BLUE}=====================================${NC}"
echo ""

echo -e "${CYAN}📧 Email Format:${NC}"
echo "  <USER_ID>@<platform>"
echo ""

if [ -n "$TELEGRAM_TOKEN" ]; then
    echo -e "${YELLOW}🔸 Telegram Examples:${NC}"
    echo "  123456789@telegram        # User ID"
    echo "  -1001234567@telegram      # Group chat"
    echo ""
fi

if [ -n "$SLACK_TOKEN" ]; then
    echo -e "${YELLOW}🔸 Slack Examples:${NC}"
    echo "  U1234567890@slack         # User ID"
    echo "  john.doe@slack            # Username (auto-resolved)"
    echo "  #general@slack            # Channel name"
    echo "  C1234567890@slack         # Channel ID"
    echo ""
fi

echo -e "${CYAN}🧪 Test Commands (Plain SMTP - Port $SMTP_PORT):${NC}"
echo ""

if [ -n "$TELEGRAM_TOKEN" ]; then
    echo -e "${GREEN}# Test Telegram:${NC}"
    echo "swaks --to 123456789@telegram --from test@$(hostname) --server localhost:$SMTP_PORT --body 'Plain SMTP test'"
    echo ""
fi

if [ -n "$SLACK_TOKEN" ]; then
    echo -e "${GREEN}# Test Slack:${NC}"
    echo "swaks --to john.doe@slack --from test@$(hostname) --server localhost:$SMTP_PORT --body 'Plain SMTP test'"
    echo "swaks --to '#general@slack' --from test@$(hostname) --server localhost:$SMTP_PORT --body 'Channel test'"
    echo ""
fi

echo -e "${CYAN}🔐 Test Commands (STARTTLS - Port $TLS_PORT):${NC}"
echo ""

if [ -n "$TELEGRAM_TOKEN" ]; then
    echo -e "${GREEN}# Test Telegram with TLS:${NC}"
    echo "swaks --to 123456789@telegram --from test@$(hostname) --server localhost:$TLS_PORT --tls --tls-no-verify --body 'STARTTLS test'"
    echo ""
fi

if [ -n "$SLACK_TOKEN" ]; then
    echo -e "${GREEN}# Test Slack with TLS:${NC}"
    echo "swaks --to john.doe@slack --from test@$(hostname) --server localhost:$TLS_PORT --tls --tls-no-verify --body 'STARTTLS test'"
    echo "swaks --to '#general@slack' --from test@$(hostname) --server localhost:$TLS_PORT --tls --tls-no-verify --body 'Secure channel test'"
    echo ""
fi

echo -e "${CYAN}📋 Other useful commands:${NC}"
echo ""
echo -e "${GREEN}# Check certificate details:${NC}"
echo "swaks --to test@telegram --from test@$(hostname) --server localhost:$TLS_PORT --tls --tls-get-peer-cert --tls-no-verify"
echo ""
echo -e "${GREEN}# Send with custom subject:${NC}"
echo "swaks --to 123456789@telegram --from monitor@$(hostname) --server localhost:$SMTP_PORT --header 'Subject: Server Alert' --body 'High CPU detected!'"
echo ""
echo -e "${GREEN}# Multiple recipients (sends to first only):${NC}"
echo "swaks --to 123456789@telegram,john.doe@slack --from test@$(hostname) --server localhost:$SMTP_PORT --body 'Multi-recipient test'"
echo ""

echo -e "${BLUE}=====================================${NC}"
echo ""

# Final confirmation
echo -e "${YELLOW}📝 Configuration Summary:${NC}"
echo "  SMTP Host: $SMTP_HOST"
echo "  Plain SMTP Port: $SMTP_PORT"
echo "  STARTTLS Port: $TLS_PORT"
echo "  ACL: 127.0.0.1/32 (localhost only)"
echo "  TLS: Enabled with self-signed certificates"
echo "  Log file: test.log"
if [ -n "$TELEGRAM_TOKEN" ]; then
    echo "  Telegram: Configured"
fi
if [ -n "$SLACK_TOKEN" ]; then
    echo "  Slack: Configured"
fi
echo ""

echo -e "${YELLOW}⚠️  Note: You'll need to replace the example IDs with real ones:${NC}"
if [ -n "$TELEGRAM_TOKEN" ]; then
    echo "  - Get your Telegram user ID from @userinfobot"
fi
if [ -n "$SLACK_TOKEN" ]; then
    echo "  - Get Slack User IDs from profile → More → Copy member ID"
    echo "  - Or use actual usernames/channel names"
fi
echo ""

echo -e "${CYAN}🚀 Ready to start email2dm!${NC}"
echo "Press Enter to continue, or Ctrl+C to exit..."
read

echo ""
echo -e "${GREEN}🚀 Starting email2dm...${NC}"
echo "Logging to: test.log"
echo "Press Ctrl+C to stop"
echo ""

# Create a separator in the log file
echo "=====================================" >> test.log
echo "email2dm started at $(date)" >> test.log
echo "Configuration:" >> test.log
echo "  SMTP_LISTEN_HOST: $SMTP_LISTEN_HOST" >> test.log
echo "  SMTP_LISTEN_PORT: $SMTP_LISTEN_PORT" >> test.log
echo "  ALLOWED_NETWORKS: $ALLOWED_NETWORKS" >> test.log
echo "  TLS_ENABLE: $TLS_ENABLE" >> test.log
echo "=====================================" >> test.log

# Start email2dm and log output
./email2dm 2>&1 | tee -a test.log
