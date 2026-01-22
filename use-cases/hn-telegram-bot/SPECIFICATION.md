# HN Telegram Bot - Complete Technical Specification

## Project Overview

Build a Telegram bot that delivers personalized daily digests of Hacker News articles. The bot learns user preferences through thumbs-up reactions and uses AI-generated summaries to provide concise article previews.

### Target Environment
- **Primary deployment**: Raspberry Pi (ARM64 architecture)
- **Designed for**: Single-user operation
- **Language**: Go 1.24+
- **Database**: SQLite (embedded, no external dependencies)

---

## Development Requirements

### Methodology
- **Use Test-Driven Development (TDD)** for all implementation
- Write failing tests first, then implement minimal code to pass
- Refactor after tests pass while maintaining test coverage
- Target **near 100% test coverage** where feasible

### Code Quality Principles
The code will be evaluated on:
- **Clarity**: Self-explanatory code with clear naming
- **DRY (Don't Repeat Yourself)**: No duplicate logic
- **SOLID principles**: Single responsibility, dependency injection, interface segregation
- **YAGNI (You Aren't Gonna Need It)**: No speculative features or over-engineering

### Execution Approach
- You are free to create plans, split work into subtasks, and spawn subagents as needed
- The goal is a fully functional application
- Code is 100% AI-generated without human intervention - optimize for AI readability and maintainability
- Use structured logging throughout for observability

---

## Architecture

### High-Level Design

The application consists of a single Go binary that coordinates several components:

1. **Telegram Bot** - Handles user commands and reactions via long polling
2. **HN Client** - Fetches top stories and item details from Hacker News API
3. **Scraper** - Extracts readable content from article URLs
4. **Summarizer** - Generates summaries and tags using Gemini AI
5. **Ranker** - Scores and ranks articles based on learned preferences
6. **Scheduler** - Triggers daily digest delivery at configured time
7. **Storage** - Persists articles, preferences, and settings to SQLite

### Package Organization

Organize code into these packages:
- `config` - Configuration loading with defaults and validation
- `storage` - Database initialization and all persistence operations
- `bot` - Telegram bot core and command handlers
- `hn` - Hacker News API client
- `scraper` - Article content extraction
- `summarizer` - Gemini API integration for summaries
- `ranker` - Article scoring algorithm
- `scheduler` - Cron-based scheduling
- `digest` - End-to-end workflow orchestration

The main package wires all components together and handles graceful shutdown.

---

## Dependencies

Use these specific Go libraries:

| Package | Purpose |
|---------|---------|
| `github.com/go-telegram-bot-api/telegram-bot-api/v5` | Telegram Bot API wrapper |
| `modernc.org/sqlite` | Pure Go SQLite (no CGO required) |
| `github.com/go-shiori/go-readability` | Article content extraction |
| `github.com/robfig/cron/v3` | Cron scheduling with timezone support |
| `gopkg.in/yaml.v3` | YAML configuration parsing |

**Important**: No external frameworks. Use standard library HTTP for Gemini API integration.

---

## Database Schema

The SQLite database requires four tables:

### Articles Table
Stores fetched HN articles with their summaries and delivery status. Fields:
- Primary key: HN item ID (integer)
- Title, URL, and AI-generated summary (text)
- Tags stored as JSON array (text field)
- HN score at fetch time (integer)
- Timestamps for when fetched and when sent to user
- Telegram message ID for correlating reactions

### Likes Table
Tracks which articles the user has liked for idempotency. Fields:
- Primary key: article ID (references articles)
- Timestamp when liked

### Tag Weights Table
Stores learned preferences for content categories. Fields:
- Primary key: tag name (text, e.g., "rust", "machine-learning")
- Weight value (real number, default 1.0)
- Occurrence count (integer, tracks how many liked articles had this tag)

### Settings Table
Key-value store for user configuration. Fields:
- Primary key: setting key (text)
- Value (text)

Used keys: `chat_id`, `digest_time`, `article_count`

---

## Configuration

### Configuration File

The bot reads configuration from a YAML file. Required fields:
- `telegram_token` - Bot token from BotFather
- `gemini_api_key` - Google AI Studio API key

Optional fields with defaults:
- `chat_id` - User's Telegram chat ID (default: 0, set via /start command)
- `gemini_model` - Gemini model to use (default: "gemini-2.0-flash-lite")
- `digest_time` - Daily digest time in HH:MM 24-hour format (default: "09:00")
- `timezone` - IANA timezone identifier (default: "UTC")
- `article_count` - Number of articles per digest (default: 30)
- `fetch_timeout_secs` - HTTP timeout for scraping (default: 10)
- `tag_decay_rate` - Percentage decay per fetch cycle (default: 0.02 = 2%)
- `min_tag_weight` - Minimum weight floor (default: 0.1)
- `tag_boost_on_like` - Weight increase per like (default: 0.2)
- `db_path` - SQLite database file path (default: "./hn-bot.db")
- `log_level` - Logging verbosity (default: "info")

### Environment Variables
- `HN_BOT_CONFIG` - Path to config file (default: "./config.yaml")
- `HN_BOT_DB` - Override database path from config

### Validation
- Fail startup if telegram_token or gemini_api_key is missing
- Time format must be valid HH:MM with hours 0-23 and minutes 0-59
- Timezone must be valid IANA identifier

---

## External API Integrations

### Hacker News API

Base URL: `https://hacker-news.firebaseio.com`

Operations needed:
1. **Get Top Stories** - Endpoint `/v0/topstories.json` returns an array of story IDs (integers)
2. **Get Item Details** - Endpoint `/v0/item/{id}.json` returns item data including: id, title, url, score, descendants (comment count), author, timestamp, type

### Telegram Bot API

Use the go-telegram-bot-api library but with custom handling for reactions:

1. **Long Polling** - Use manual `getUpdates` calls (not the library's built-in polling) to support `message_reaction` events
2. **Allowed Updates** - Request both `message` and `message_reaction` update types
3. **Message Sending** - Send article messages with HTML formatting
4. **Reaction Events** - The standard library doesn't support message reactions. Create custom types to unmarshal the `message_reaction` field from update responses. The reaction data includes: chat info, message ID, user, date, old reactions array, new reactions array. Each reaction has a type and emoji field.

### Gemini API

Use direct HTTP calls to the Gemini REST API (no SDK).

Endpoint: `https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent`

Request: POST with JSON body containing contents array with parts containing text.

Response: Parse candidates array, extract text from first candidate's content parts.

**Summarization Prompt**: Ask Gemini to summarize the article in 1-2 sentences and provide 3-5 lowercase tags categorizing the topic. Request JSON response format with "summary" and "tags" fields.

**Response Handling**: Gemini may wrap JSON in markdown code blocks. Strip these before parsing.

---

## Features and Behavior

### Bot Commands

#### /start Command
- Saves the sender's chat ID to the database settings
- Sends a welcome message listing available commands
- Must be called before other commands work (if chat_id not in config)

#### /fetch Command
- Manually triggers the digest workflow immediately
- Runs the complete pipeline: fetch stories, scrape content, summarize, rank, send
- Sends articles directly without confirmation message

#### /settings Command
- Without arguments: displays current digest time and article count
- With "time HH:MM": validates and updates digest time, persists to database, updates scheduler
- With "count N": validates (1-100 range) and updates article count, persists to database
- Invalid input returns usage instructions

#### /stats Command
- Retrieves top 10 tags by weight with their current weights
- Retrieves total like count
- If no likes yet, sends prompt to use thumbs-up reactions
- Otherwise displays formatted list of interests and total likes

### Reaction Handling

**Trigger**: User adds thumbs-up emoji (👍) to a bot message

**Process**:
1. Look up the article by the Telegram message ID
2. If no article found for that message ID, ignore (not an article message)
3. Check if article is already liked - if so, ignore (idempotent)
4. Boost weight of each tag from the article by the configured boost amount
5. Record the like in the likes table with timestamp

**Other Reactions**: Any emoji other than thumbs-up is silently ignored.

### Daily Digest Workflow

**Trigger**: Cron job fires at the configured digest_time in the configured timezone

**Pipeline**:

1. **Apply Decay** - Reduce all tag weights by the decay rate, with minimum floor. Formula: new_weight = max(current_weight × (1 - decay_rate), min_weight)

2. **Fetch Stories** - Get 2× the desired article count from HN top stories (extra buffer for filtering)

3. **Filter Recent** - Exclude any articles that were sent to the user in the last 7 days

4. **Process Each Story**:
   - Fetch item details from HN API
   - Scrape article content using readability library (limit to 4000 characters)
   - If scraping fails, use the article title as fallback content
   - Call Gemini to generate summary and tags
   - If summarization fails, skip the article and continue

5. **Rank Articles** - Score each article using learned preferences:
   - Tag score = sum of weights for all article tags
   - HN score component = log10(article_score + 1)
   - Final score = (tag_score × 0.7) + (hn_score × 0.3)
   - Sort articles by final score descending

6. **Send Top N** - Send the highest-scored articles up to the configured count

7. **Persist** - For each sent article: save to database, mark as sent with timestamp, store the Telegram message ID for reaction tracking

### Preference Learning Algorithm

**On Like (thumbs-up reaction)**:
For each tag in the liked article's tags:
- If tag doesn't exist in tag_weights, insert with weight = 1.0 + boost_amount
- If tag exists, add boost_amount to current weight
- Increment the tag's occurrence counter

**Decay (on each fetch cycle)**:
Apply to all tags: multiply weight by (1 - decay_rate), but never go below min_weight

This creates recency-biased learning where recent interests have higher weights than older ones.

---

## Message Formatting

### Article Message Format

Use HTML formatting mode. Each article message should display:
- Article icon and title in bold
- Summary in italics
- Score (points) and comment count with icons
- Links to both the original article and the HN discussion page

HN discussion URL format: `https://news.ycombinator.com/item?id={article_id}`

**HTML Escaping**: The title and summary text must have HTML special characters escaped (ampersand, less-than, greater-than).

### Command Response Messages

- **/start**: Welcome message mentioning /fetch, /settings, and /stats commands
- **/settings** (display): Show current digest time and article count
- **/settings** (update): Confirm the updated value
- **/stats** (with data): List top tags with weights, total like count
- **/stats** (no data): Encourage user to react with thumbs-up to train preferences

---

## Error Handling and Resilience

### Graceful Degradation
- **Scraper fails**: Use article title as content instead, continue processing
- **Gemini fails**: Skip that article, continue with remaining articles
- **Send message fails**: Log error, continue to next article
- **Database write fails**: Log warning, don't crash the application

### Startup Validation
- Exit with error if required config fields are missing
- Initialize database with schema (create tables if not exist)
- Validate timezone string loads correctly

### Graceful Shutdown
- Catch SIGINT and SIGTERM signals
- Cancel context to stop in-flight operations
- Stop the scheduler
- Close database connection
- Exit cleanly

---

## Logging

Use Go's structured logging package (slog) with JSON output to stdout.

### Log Points
- **Startup**: Config loaded, each component initialized, scheduler started
- **Digest cycle**: Start, article counts at each stage, completion
- **API calls**: Failures with relevant context (IDs, URLs, error details)
- **Reactions**: Message ID, found article ID, tags boosted
- **Settings changes**: What changed and to what value
- **Errors**: Full context for debugging

---

## Interface Design

Design with dependency injection and interface segregation in mind:

- Each component should depend on narrow interfaces, not concrete types
- The digest workflow needs interfaces for: HN client, scraper, summarizer, storage, article sender
- Command handlers need interfaces for: message sending, storage operations
- The reaction handler needs interfaces for: article lookup, like tracking, tag boosting
- The settings handler needs interfaces for: message sending, settings storage, schedule updating

This enables unit testing with mock implementations.

---

## Build Requirements

### Build Targets
- Default build: produces binary for current platform
- ARM64 build: cross-compile for Linux ARM64 (Raspberry Pi)
- Test: run all tests
- Coverage: generate coverage report

### Binary
Single static executable with SQLite embedded (no external runtime dependencies).

---

## Testing Requirements

### Coverage Target
Aim for near 100% test coverage. Every public function should have tests covering both success and error cases.

### Testing Approach
- Use mock implementations of interfaces for unit tests
- Use table-driven tests for validation logic
- Test full pipelines with mocked external services
- Each source file should have a corresponding test file

### Key Areas to Test
1. **Configuration**: Loading, defaults, validation, environment variable overrides
2. **Storage**: All CRUD operations, upsert behavior, query filtering
3. **Bot Commands**: Each handler's response and state changes
4. **Reactions**: Tag boosting logic, idempotency, emoji filtering
5. **Ranker**: Scoring formula correctness, sort order
6. **Scheduler**: Cron expression generation, time parsing, timezone handling
7. **Summarizer**: JSON parsing, markdown code block stripping, error cases
8. **Workflow**: Complete pipeline with mocked dependencies

---

## Design Decisions (Preserve These)

These are intentional architectural choices:

1. **Single-user only** - No multi-user support needed. One chat_id stored.
2. **Long polling** - Not webhooks. Simpler deployment without exposing ports.
3. **Pure Go SQLite** - Using modernc.org/sqlite for easy cross-compilation.
4. **Synchronous pipeline** - No async message queues. Sequential processing.
5. **Tag decay on fetch** - Not time-based. Avoids background goroutines.
6. **Direct HTTP for Gemini** - No official SDK. Simpler implementation.
7. **70/30 blended ranking** - Learned preferences dominate but HN score provides baseline quality signal.
8. **Reaction idempotency** - Multiple thumbs-up on same article don't compound the boost.
9. **7-day recency filter** - Recently sent articles excluded from future digests.
10. **Thread-safe settings** - Use mutex for concurrent access to in-memory settings state.

---

## Success Criteria

The implementation is complete when:

1. All tests pass with high coverage
2. Bot successfully:
   - Registers users via /start
   - Fetches and sends personalized digests on schedule
   - Learns from thumbs-up reactions to improve recommendations
   - Allows runtime settings updates
   - Displays preference statistics
3. Code follows DRY, SOLID, YAGNI principles
4. All external API integrations work correctly
5. Graceful shutdown handles signals properly
6. ARM64 binary cross-compiles successfully

---

## Notes for AI Agents

- **Parallelization**: Feel free to split into subtasks and work on independent packages concurrently
- **Interface-first**: Define interfaces before implementations to enable proper TDD
- **Mocking strategy**: Create mock types in test files as needed
- **Error handling**: Return errors up the call stack, don't panic. Log context at appropriate levels.
- **No magic**: Explicit is better than implicit. Clear function signatures and data flow.
- **Testability**: Design for testability from the start. All dependencies should be injectable.
