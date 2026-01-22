# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

This repository evaluates how different AI coding agents perform on the same implementation task. The primary use case is a Hacker News Telegram Bot specified in `use-cases/hn-telegram-bot/SPECIFICATION.md`. Multiple implementations exist from different AI agents (Claude Code, OpenCode with various models).

## Repository Structure

```
/use-cases/hn-telegram-bot/
├── SPECIFICATION.md          # Complete technical requirements (canonical source of truth)
├── EVALUATION_PROMPT.md      # Scoring framework for implementations
├── results-summary.md        # Comparative analysis across implementations
├── claude-code-opus-4.5/     # Implementation by Claude Code (Opus 4.5)
├── opencode-gemini-3-*/      # Implementations by OpenCode with Gemini models
├── opencode-minimax-m2.1/    # Implementation by OpenCode with Minimax
├── opencode-glm-4.7/         # Implementation by OpenCode with GLM
├── opencode-grok-code-fast-1/# Implementation by OpenCode with Grok
└── EVALUATION_*.md           # Evaluation reports for each implementation
```

## Working with Implementations

Each implementation folder contains a complete Go project:

### Common Commands (within any implementation folder)

```bash
make build           # Build binary for current platform
make build-arm64     # Cross-compile for Linux ARM64 (Raspberry Pi target)
make test            # Run all tests
make coverage        # Generate coverage report
make clean           # Clean build artifacts

# Run single test
go test -v ./path/to/package -run TestName
```

### Go Module

- Language: Go 1.24+
- Module name: `hn-telegram-bot`
- Uses pure Go SQLite (`modernc.org/sqlite`) for cross-compilation without CGO

### Package Structure

Each implementation follows the same package organization:
- `config` - YAML configuration with validation
- `storage` - SQLite persistence (4 tables: articles, likes, tag_weights, settings)
- `bot` - Telegram bot commands (/start, /fetch, /settings, /stats)
- `hn` - Hacker News API client
- `scraper` - Article content extraction
- `summarizer` - Gemini AI integration (direct HTTP, no SDK)
- `ranker` - 70% learned preference + 30% HN score formula
- `scheduler` - Cron-based daily digest
- `digest` - Workflow orchestration

## Development Principles

From the specification, implementations must follow:

1. **TDD Mandatory** - Write failing tests first, target near 100% coverage
2. **Interface-Driven** - All dependencies via interfaces for testability
3. **Graceful Degradation** - Don't crash on external API failures
4. **Single-User Design** - No multi-user support (intentional simplification)
5. **Long Polling** - Not webhooks (simpler ARM deployment)

## Evaluation Criteria

Implementations are scored on 10 categories (0-10 each, 100 total):
- Functional Completeness
- Test Quality & Coverage
- Code Clarity
- DRY Principle
- SOLID Principles
- YAGNI Principle
- Error Handling & Resilience
- Interface Design & Testability
- Logging & Observability
- Specification Adherence

## Creating New Evaluations

1. Create a new implementation folder under `use-cases/hn-telegram-bot/`
2. Implement according to `SPECIFICATION.md`
3. Use `EVALUATION_PROMPT.md` as the scoring rubric
4. Document results in `EVALUATION_<agent-name>.md`
5. Update `results-summary.md` with comparative data
