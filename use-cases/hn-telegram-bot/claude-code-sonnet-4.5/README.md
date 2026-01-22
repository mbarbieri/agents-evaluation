# Hacker News Telegram Bot

A Telegram bot that delivers personalized daily digests of Hacker News articles. The bot learns your preferences through thumbs-up reactions and uses AI-generated summaries to provide concise article previews.

## Features

- **Personalized Recommendations**: Learns your preferences from thumbs-up reactions
- **AI Summaries**: Uses Gemini AI to generate concise article summaries with topic tags
- **Smart Ranking**: Combines learned preferences (70%) with HN score (30%) for optimal article selection
- **Daily Digests**: Scheduled delivery at your preferred time
- **Interactive Commands**: Manage settings and view your learned interests
- **Preference Decay**: Gradually reduces tag weights to adapt to changing interests

## Requirements

- Go 1.24+
- Telegram Bot Token (from [@BotFather](https://t.me/BotFather))
- Gemini API Key (from [Google AI Studio](https://aistudio.google.com/apikey))

## Installation

### From Source

```bash
# Clone the repository
git clone <repository-url>
cd hn-telegram-bot

# Copy example config
cp config.yaml.example config.yaml

# Edit config.yaml with your tokens
nano config.yaml

# Build for current platform
make build

# Or build for Raspberry Pi (ARM64)
make build-arm64
```

## Configuration

Create a `config.yaml` file based on `config.yaml.example`:

```yaml
telegram_token: "your_bot_token"
gemini_api_key: "your_gemini_key"
digest_time: "09:00"
timezone: "America/New_York"
article_count: 30
```

### Configuration Options

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `telegram_token` | **Yes** | - | Bot token from @BotFather |
| `gemini_api_key` | **Yes** | - | Gemini API key from Google AI Studio |
| `chat_id` | No | 0 | Auto-set on /start command |
| `gemini_model` | No | gemini-2.0-flash-lite | Gemini model to use |
| `digest_time` | No | 09:00 | Daily digest time (HH:MM) |
| `timezone` | No | UTC | IANA timezone identifier |
| `article_count` | No | 30 | Articles per digest |
| `fetch_timeout_secs` | No | 10 | Article scraping timeout |
| `tag_decay_rate` | No | 0.02 | Tag weight decay per cycle (2%) |
| `min_tag_weight` | No | 0.1 | Minimum tag weight floor |
| `tag_boost_on_like` | No | 0.2 | Weight boost per like |
| `db_path` | No | ./hn-bot.db | SQLite database path |
| `log_level` | No | info | Logging level (debug/info/warn/error) |

### Environment Variables

- `HN_BOT_CONFIG`: Path to config file (default: `./config.yaml`)
- `HN_BOT_DB`: Override database path from config

## Usage

### Starting the Bot

```bash
./hn-bot
```

### Bot Commands

#### `/start`
Register with the bot and get a welcome message. This saves your chat ID for digest delivery.

```
/start
```

#### `/fetch`
Manually trigger a digest delivery immediately (bypasses schedule).

```
/fetch
```

#### `/settings`
View or update bot settings.

**View current settings:**
```
/settings
```

**Update digest time:**
```
/settings time 14:30
```

**Update article count:**
```
/settings count 50
```

#### `/stats`
View your learned preferences and interests.

```
/stats
```

### Training Preferences

React with 👍 (thumbs-up) to articles you enjoy. The bot will:
1. Record the like
2. Boost the weight of all tags from that article
3. Prioritize similar content in future digests

### How It Works

1. **Daily Workflow**:
   - Applies tag weight decay (recency bias)
   - Fetches top HN stories (2× desired count for filtering)
   - Filters out recently sent articles (7-day window)
   - Scrapes article content and generates AI summaries
   - Ranks articles using learned preferences and HN scores
   - Sends top N articles to your chat

2. **Ranking Algorithm**:
   ```
   tag_score = sum(weights for all article tags)
   hn_score = log10(hn_points + 1)
   final_score = (tag_score × 0.7) + (hn_score × 0.3)
   ```

3. **Preference Learning**:
   - Each thumbs-up boosts related tag weights by 0.2
   - Weights decay by 2% each fetch cycle
   - Minimum weight floor of 0.1 prevents complete decay

## Development

### Run Tests

```bash
make test
```

### Generate Coverage Report

```bash
make coverage
```

### Build Targets

```bash
make build           # Build for current platform
make build-arm64     # Cross-compile for Linux ARM64
make test            # Run all tests
make coverage        # Generate coverage report
make clean           # Clean build artifacts
```

## Deployment

### Raspberry Pi

1. Build ARM64 binary on your development machine:
   ```bash
   make build-arm64
   ```

2. Transfer to Raspberry Pi:
   ```bash
   scp hn-bot-arm64 pi@raspberrypi:~/hn-bot
   scp config.yaml pi@raspberrypi:~/
   ```

3. Run on Pi:
   ```bash
   ssh pi@raspberrypi
   chmod +x ~/hn-bot
   ~/hn-bot
   ```

### Running as a Service

Create a systemd service file `/etc/systemd/system/hn-bot.service`:

```ini
[Unit]
Description=HN Telegram Bot
After=network.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi
ExecStart=/home/pi/hn-bot
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable hn-bot
sudo systemctl start hn-bot
sudo systemctl status hn-bot
```

## Architecture

The application consists of these packages:

- **config**: Configuration loading and validation
- **storage**: SQLite persistence layer
- **bot**: Telegram bot commands and reaction handling
- **hn**: Hacker News API client
- **scraper**: Article content extraction
- **summarizer**: Gemini AI integration for summaries
- **ranker**: Article scoring algorithm
- **scheduler**: Cron-based scheduling
- **digest**: End-to-end workflow orchestration

## Database Schema

### Articles Table
Stores fetched articles with summaries and delivery status.

### Likes Table
Tracks user reactions for idempotency.

### Tag Weights Table
Stores learned preference weights for content categories.

### Settings Table
Key-value store for user configuration.

## License

[Your License Here]

## Contributing

[Your Contributing Guidelines Here]
