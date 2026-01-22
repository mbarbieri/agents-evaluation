# HN Telegram Bot

A Telegram bot that delivers personalized daily digests of Hacker News articles using AI summarization and user preference learning.

## Features

- **Personalized Digests**: Learns your interests based on thumbs-up reactions.
- **AI Summaries**: Uses Google Gemini to generate concise summaries and tags.
- **Smart Ranking**: Ranks articles by combining HN score and learned tag weights.
- **Readable Content**: Scrapes and extracts main article content.
- **Cron Scheduling**: Configurable daily delivery time.
- **Persistence**: SQLite database for storing history and preferences.
- **Single Binary**: Easy deployment (Raspberry Pi friendly).

## Configuration

Create a `config.yaml` file:

```yaml
telegram_token: "YOUR_BOT_TOKEN"
gemini_api_key: "YOUR_GEMINI_KEY"
chat_id: 0 # Will be set automatically on /start
digest_time: "09:00" # HH:MM
timezone: "America/New_York"
article_count: 30
```

## Running

### Build

```bash
go build -o hn-bot cmd/bot/main.go
```

### Run

```bash
export HN_BOT_CONFIG=./config.yaml
./hn-bot
```

### Commands

- `/start` - Initialize the bot
- `/fetch` - Trigger an immediate digest
- `/settings` - View/Edit settings (e.g., `/settings time 08:00`)
- `/stats` - View your top interests and total likes

## Development

Run tests:
```bash
go test ./...
```
