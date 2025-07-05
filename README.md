# SMTP to Telegram Bridge

A lightweight, secure SMTP server that forwards emails to Telegram chats. Perfect for automated systems, monitoring alerts, and notification delivery.

## 🚀 Features

- **Dynamic Telegram Routing**: Extract Telegram chat ID from email address (`123456789@example.com`)
- **STARTTLS Support**: Optional TLS encryption with backward compatibility
- **Network ACLs**: IP-based access control using CIDR notation
- **Message Splitting**: Automatically handles long messages within Telegram's limits
- **Syslog Integration**: Comprehensive logging of all email processing events
- **Multi-recipient Support**: Single bridge instance serves multiple Telegram destinations
- **Production Ready**: Built for reliability with proper error handling

## 📧 How It Works

Send emails to: `<TELEGRAM_ID>@<any-domain>`

**Examples:**
- `123456789@notifications.company.com` → Sends to Telegram user ID 123456789
- `-1001234567@alerts.company.com` → Sends to Telegram group chat -1001234567

## 🔧 Installation

### Prerequisites
- Go 1.19+ (for building from source)
- Telegram bot token (get from [@BotFather](https://t.me/BotFather))

### Build from Source
```bash
git clone <repository-url>
cd smtp-telegram-bridge
go mod init smtp-telegram-bridge
go get github.com/emersion/go-smtp
go build -o smtp-telegram-bridge
```

### Quick Start
```bash
# Set your bot token
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

# Run the bridge
./smtp-telegram-bridge
```

## ⚙️ Configuration

### Required Environment Variables
| Variable | Description |
|----------|-------------|
| `TELEGRAM_BOT_TOKEN` | Your Telegram bot token from @BotFather |

### Optional Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `SMTP_LISTEN_HOST` | `0.0.0.0` | IP address to bind SMTP server |
| `SMTP_LISTEN_PORT` | `2525` | Port for SMTP server |
| `ALLOWED_NETWORKS` | _(none)_ | Comma-separated CIDR networks (e.g., `192.168.1.0/24,10.0.0.0/8`) |
| `TLS_ENABLE` | `false` | Enable STARTTLS support (`true`/`false`) |
| `TLS_CERT_PATH` | _(none)_ | Path to TLS certificate file (required if TLS enabled) |
| `TLS_KEY_PATH` | _(none)_ | Path to TLS private key file (required if TLS enabled) |

## 🔒 Security Features

### Network Access Control Lists (ACLs)
Restrict which IP addresses can connect to the SMTP server:

```bash
# Allow only local networks
export ALLOWED_NETWORKS="192.168.1.0/24,10.0.0.0/8,127.0.0.1/32"
```

### TLS/STARTTLS Support
Enable encrypted email transmission:

```bash
export TLS_ENABLE="true"
export TLS_CERT_PATH="/path/to/server.crt"
export TLS_KEY_PATH="/path/to/server.key"
```

**Note**: STARTTLS allows both encrypted and unencrypted connections on the same port for maximum compatibility.

## 📋 Usage Examples

### Basic Setup (Plain SMTP)
```bash
export TELEGRAM_BOT_TOKEN="your_bot_token_here"
export SMTP_LISTEN_HOST="127.0.0.1"    # Localhost only
export SMTP_LISTEN_PORT="2525"
./smtp-telegram-bridge
```

### Production Setup (with TLS and ACLs)
```bash
export TELEGRAM_BOT_TOKEN="your_bot_token_here"
export SMTP_LISTEN_PORT="587"
export TLS_ENABLE="true"
export TLS_CERT_PATH="/etc/ssl/certs/mail.crt"
export TLS_KEY_PATH="/etc/ssl/private/mail.key"
export ALLOWED_NETWORKS="10.0.0.0/8,192.168.0.0/16"
./smtp-telegram-bridge
```

### Generating Self-Signed Certificates
```bash
# Generate private key
openssl genrsa -out server.key 2048

# Generate certificate
openssl req -new -x509 -key server.key -out server.crt -days 365 -subj "/CN=localhost"
```

## 🧪 Testing

### Using swaks (recommended)
```bash
# Install swaks
sudo apt-get install swaks  # Ubuntu/Debian
brew install swaks          # macOS

# Test plain SMTP
swaks --to 123456789@example.com --from test@company.com --server localhost:2525 --body "Test message"

# Test with STARTTLS
swaks --to 123456789@example.com --from test@company.com --server localhost:587 --tls --body "Encrypted test"
```

### Using telnet
```bash
telnet localhost 2525
EHLO test
MAIL FROM:<test@company.com>
RCPT TO:<123456789@example.com>
DATA
Subject: Test Message

Hello from SMTP bridge!
.
QUIT
```

## 📊 Logging

All email processing events are logged to syslog with the format:
```
src=<source_ip> from=<sender_email> telegram_id=<chat_id> msg=<status>
```

**Example log entries:**
```
src=192.168.1.100 from=monitor@company.com telegram_id=123456789 msg=Processing email
src=192.168.1.100 from=monitor@company.com telegram_id=123456789 msg=Email sent successfully
src=1.2.3.4 from=spam@bad.com telegram_id=999999999 msg=Telegram send failed: 401 Unauthorized
```

## 🎯 Use Cases

### Server Monitoring
```bash
# Send server alerts to your personal Telegram
echo "High CPU usage detected!" | mail -s "Server Alert" 123456789@alerts.company.com
```

### Application Notifications
```bash
# Send deployment notifications to team chat
curl -X POST localhost:2525 -d "Subject: Deployment Complete
To: -1001234567@deploy.company.com

Application v2.1.0 deployed successfully!"
```

### Automated Backup Reports
```bash
# Daily backup status to admin group
echo "Backup completed: 500GB transferred" | mail -s "Daily Backup" -1001234567@backups.company.com
```

## 🔍 Troubleshooting

### Invalid Bot Token
- **Symptom**: Emails accepted but not delivered to Telegram
- **Solution**: Check syslog for `401 Unauthorized` errors, verify bot token

### Network Connection Rejected
- **Symptom**: SMTP connection immediately drops
- **Solution**: Check `ALLOWED_NETWORKS` configuration, verify source IP

### TLS Certificate Errors
- **Symptom**: `TLS certificate file not found` on startup
- **Solution**: Verify certificate paths exist and are readable

### Telegram Rate Limiting
- **Symptom**: `429 Too Many Requests` in syslog
- **Solution**: Reduce email frequency, messages are automatically delayed

## 🆘 Help

```bash
./smtp-telegram-bridge --help
```

## 📜 License

[Add your license here]

## 🤝 Contributing

[Add contribution guidelines here]