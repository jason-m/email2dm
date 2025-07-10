# email2dm - SMTP to Chat Platform Bridge

A lightweight, secure SMTP server that forwards emails to multiple chat platforms. Perfect for automated systems, monitoring alerts, and notification delivery.

## 🚀 Features

- **Multi-Platform Support**: Telegram, Slack, and easily extensible to other platforms
- **Dynamic Platform Routing**: Extract platform and user ID from email address (`123456789@telegram`)
- **Username Resolution**: Automatic Slack username-to-ID lookup with intelligent caching
- **STARTTLS Support**: Optional TLS encryption with backward compatibility
- **Network ACLs**: IP-based access control using CIDR notation
- **Message Splitting**: Automatically handles long messages within each platform's limits
- **Syslog Integration**: Comprehensive logging of all email processing events
- **Production Ready**: Built for reliability with proper error handling and performance optimization

## 📧 How It Works

Send emails to: `<USER_ID>@<platform>`

**Telegram Examples:**
- `123456789@telegram` → Sends to Telegram user ID 123456789
- `-1001234567@telegram` → Sends to Telegram group chat -1001234567

**Slack Examples:**
- `U1234567890@slack` → Sends to Slack user ID U1234567890
- `C1234567890@slack` → Sends to Slack channel ID C1234567890
- `#general@slack` → Sends to Slack channel #general
- `john.doe@slack` → Sends to Slack user by username (auto-resolved to User ID)

## 🔧 Installation

### Prerequisites
- Go 1.19+ (for building from source)
- At least one platform bot token:
  - Telegram bot token (get from [@BotFather](https://t.me/BotFather))
  - Slack bot token (get from [Slack API](https://api.slack.com/apps))

### Slack Bot Setup
For full functionality, your Slack bot needs these OAuth scopes:
- `chat:write` - Send messages to channels/users
- `chat:write.public` - Send messages to public channels without joining
- `users:read` - Required for username-to-ID resolution
- `im:write` - Send direct messages to users

### Build from Source
```bash
git clone <repository-url>
cd email2dm
go mod init email2dm
go get github.com/emersion/go-smtp
go build -o email2dm
```

### Quick Start
```bash
# Set at least one platform token
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
export SLACK_BOT_TOKEN="xoxb-1234567890-1234567890-abcdefghij"

# Run the bridge
./email2dm
```

## ⚙️ Configuration

### Required Environment Variables
At least one platform token is required:

| Variable | Description |
|----------|-------------|
| `TELEGRAM_BOT_TOKEN` | Your Telegram bot token from @BotFather |
| `SLACK_BOT_TOKEN` | Your Slack bot token (xoxb-...) with required scopes |

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
export TELEGRAM_BOT_TOKEN="your_telegram_token_here"
export SLACK_BOT_TOKEN="your_slack_token_here"
export SMTP_LISTEN_HOST="127.0.0.1"    # Localhost only
export SMTP_LISTEN_PORT="2525"
./email2dm
```

### Production Setup (with TLS and ACLs)
```bash
export TELEGRAM_BOT_TOKEN="your_telegram_token_here"
export SLACK_BOT_TOKEN="your_slack_token_here"
export SMTP_LISTEN_PORT="587"
export TLS_ENABLE="true"
export TLS_CERT_PATH="/etc/ssl/certs/mail.crt"
export TLS_KEY_PATH="/etc/ssl/private/mail.key"
export ALLOWED_NETWORKS="10.0.0.0/8,192.168.0.0/16"
./email2dm
```

### Single Platform Setup
```bash
# Telegram only
export TELEGRAM_BOT_TOKEN="your_telegram_token_here"
./email2dm

# Slack only
export SLACK_BOT_TOKEN="your_slack_token_here"
./email2dm
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
nix-shell -p swaks          # NixOS 

# Test Telegram
swaks --to 123456789@telegram --from test@company.com --server localhost:2525 --body "Test message"

# Test Slack (multiple formats)
swaks --to U1234567@slack --from test@company.com --server localhost:2525 --body "User ID format"
swaks --to john.doe@slack --from test@company.com --server localhost:2525 --body "Username format (auto-resolved)"
swaks --to "#general@slack" --from test@company.com --server localhost:2525 --body "Channel name format"

# Test with STARTTLS
swaks --to 123456789@telegram --from test@company.com --server localhost:587 --tls --body "Encrypted test"
```

### Using telnet
```bash
telnet localhost 2525
EHLO test
MAIL FROM:<test@company.com>
RCPT TO:<123456789@telegram>
DATA
Subject: Test Message

Hello from email2dm!
.
QUIT
```

### Testing Username Resolution
```bash
# First time: API call to resolve username
swaks --to john.doe@slack --from test@company.com --server localhost:2525 --body "First message (resolves username)"

# Second time: uses cached User ID (much faster)
swaks --to john.doe@slack --from test@company.com --server localhost:2525 --body "Second message (uses cache)"
```

## 📊 Logging

All email processing events are logged to syslog with the format:
```
src=<source_ip> from=<sender_email> platform=<platform> user_id=<chat_id> msg=<status>
```

**Example log entries:**
```
src=192.168.1.100 from=monitor@company.com platform=telegram user_id=123456789 msg=Processing email
src=192.168.1.100 from=monitor@company.com platform=slack user_id=U1234567 msg=Email sent successfully
src=192.168.1.100 from=monitor@company.com platform=slack user_id=john.doe msg=Resolved username john.doe to User ID U1234567
src=1.2.3.4 from=spam@bad.com platform=telegram user_id=999999999 msg=Send failed: 401 Unauthorized
```

## 🎯 Use Cases

### Server Monitoring
```bash
# Send server alerts to your personal Telegram
echo "High CPU usage detected!" | mail -s "Server Alert" 123456789@telegram

# Send alerts to Slack channel by name
echo "Database backup failed!" | mail -s "Critical Alert" "#alerts@slack"

# Send alerts to specific user by username
echo "Your job failed!" | mail -s "Job Alert" john.doe@slack
```

### Application Notifications
```bash
# Send deployment notifications to team chat
curl -X POST localhost:2525 -d "Subject: Deployment Complete
To: -1001234567@telegram

Application v2.1.0 deployed successfully!"
```

### Automated Backup Reports
```bash
# Daily backup status to admin group
echo "Backup completed: 500GB transferred" | mail -s "Daily Backup" "#ops@slack"
```

### Legacy Hardware Integration
```bash
# Ancient UPS units, old network switches, vintage servers, and other
# crusty hardware that only knows SMTP but desperately wants to tell you
# when things are going sideways
echo "UPS battery low, runtime: 5 minutes" | mail -s "Hardware Alert" 123456789@telegram
```

### Cross-Platform Notifications
```bash
# Send the same alert to multiple platforms
echo "Critical system failure!" | mail -s "URGENT" 123456789@telegram
echo "Critical system failure!" | mail -s "URGENT" "#incidents@slack"
```

## 🚀 Performance Features

### Username Caching
- **First lookup**: `john.doe@slack` triggers API call to resolve username to User ID
- **Subsequent lookups**: Uses cached User ID for instant resolution
- **Bulk caching**: Single API call caches all workspace users
- **Persistence**: Cache lasts for application lifetime

### Message Optimization
- **Platform-aware splitting**: Respects each platform's message limits (Telegram: 4KB, Slack: 40KB)
- **Smart formatting**: HTML for Telegram, Markdown for Slack
- **Rate limiting**: Automatic delays between message chunks

## 🔍 Troubleshooting

### Invalid Bot Token
- **Symptom**: Emails accepted but not delivered
- **Solution**: Check syslog for `401 Unauthorized` errors, verify bot tokens

### Slack Permission Issues
- **Symptom**: `missing_scope` errors
- **Solution**: Add required OAuth scopes (`chat:write`, `users:read`, `im:write`) and reinstall app

### Username Resolution Failed
- **Symptom**: `user 'username' not found` errors
- **Solution**: Verify username exists in Slack workspace, check bot has `users:read` scope

### Platform Not Configured
- **Symptom**: `platform client not configured` errors
- **Solution**: Ensure the appropriate `*_BOT_TOKEN` environment variable is set

### Network Connection Rejected
- **Symptom**: SMTP connection immediately drops
- **Solution**: Check `ALLOWED_NETWORKS` configuration, verify source IP

### TLS Certificate Errors
- **Symptom**: `TLS certificate file not found` on startup
- **Solution**: Verify certificate paths exist and are readable

### Rate Limiting
- **Symptom**: `429 Too Many Requests` in syslog
- **Solution**: Reduce email frequency, messages are automatically delayed

### Invalid Platform ID Format
- **Symptom**: `invalid ID format` errors
- **Solution**: 
  - **Telegram**: Use numeric IDs (123456789 for users, -1001234567 for groups)
  - **Slack**: Use User IDs (`U1234567`), Channel IDs (`C1234567`), channel names (`#channel`), or usernames (`john.doe`)

## 🆘 Help

```bash
./email2dm --help
```

## 🤝 Supported Platforms

### Currently Supported
- **Telegram**: User chats, group chats, channels
  - User ID format: `123456789@telegram`
  - Group format: `-1001234567@telegram`
- **Slack**: Users, channels (by ID or name), direct messages
  - User ID format: `U1234567890@slack`
  - Channel ID format: `C1234567890@slack`
  - Channel name format: `#general@slack`
  - Username format: `john.doe@slack` (auto-resolved)

### Coming Soon
- Discord
- Microsoft Teams
- Mattermost

### Platform-Specific Features
| Feature | Telegram | Slack |
|---------|----------|-------|
| User IDs | ✅ Numeric | ✅ U-prefixed |
| Channel IDs | ✅ Negative numeric | ✅ C-prefixed |
| Channel names | ❌ | ✅ #-prefixed |
| Username resolution | ❌ | ✅ Automatic |
| Message limits | 4,096 chars | 40,000 chars |
| Formatting | HTML | Markdown |

## 📜 License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Adding New Platforms

The architecture makes it easy to add new platforms:

1. **Create client file**: `platforms/newplatform.go`
2. **Add to config**: Update environment variable parsing
3. **Add validation**: Update `validateIDForPlatform()`
4. **Add routing**: Update `sendToPlatform()`
5. **Add formatting**: Update `formatMessageForPlatform()`

See existing Telegram and Slack implementations as examples.