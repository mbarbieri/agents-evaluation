# Codebase Evaluation Report

## Project: HN Telegram Bot
## Date: 2026-01-21
## Code Location: opencode-minimax-m2.1

---

## Scores

| Category | Score |
|----------|-------|
| Functional Completeness | 5/10 |
| Test Quality & Coverage | 5/10 |
| Code Clarity | 8/10 |
| DRY Principle | 7/10 |
| SOLID Principles | 4/10 |
| YAGNI Principle | 9/10 |
| Error Handling & Resilience | 7/10 |
| Interface Design & Testability | 3/10 |
| Logging & Observability | 8/10 |
| Specification Adherence | 5/10 |
| **Total** | **61/100** |

---

## Justifications

### 1. Functional Completeness: 5/10
**Critical missing feature: Reaction handling is not implemented.** The `handleUpdates` function in main.go:154-163 only checks for `update.Message` but never processes `message_reaction` events. While `bot.IsThumbsUpReaction()` and `ReactionData` types exist, they are never called. This means the core preference learning mechanism (likes boosting tag weights) does not work. The spec explicitly requires reaction handling with thumbs-up detection and preference boosting.

All 4 commands are implemented (`/start`, `/fetch`, `/settings`, `/stats`), all 9 packages exist, database schema has all 4 tables, and graceful shutdown works. However, without reaction handling, the personalization feature is non-functional.

### 2. Test Quality & Coverage: 5/10
Overall test coverage is **58.6%**, far from the spec's "near 100%" target. Critical gaps:
- **No test file for `digest` package** - the main workflow orchestrator is untested
- **No test file for `main` package**
- Several utility functions have 0% coverage (`RoundToDecimal`, `SentAtTime`, `MessageID`)

Positive: Table-driven tests exist in config, bot, ranker, and scheduler. Mock servers are used effectively for HTTP-based tests. Each package that has tests covers both success and error paths.

### 3. Code Clarity: 8/10
Code is generally well-structured and readable. Function names clearly express intent (e.g., `DecayAllTags`, `BoostTags`, `GetRecentArticles`). Functions are appropriately sized with linear control flow. Package organization follows the specification exactly. Minor deduction for the digest.go:293-302 `FormatArticleMessage` always passing 0 for comment count.

### 4. DRY Principle: 7/10
Most patterns are well-extracted. Some duplication exists:
- Time validation logic duplicated between `config/config.go:107-126` and `bot/bot.go:141-159`
- Article scanning from database rows is repeated in `storage.go` across multiple functions (lines 137-163, 175-198, 216-244)
- Error handling patterns are consistent

### 5. SOLID Principles: 4/10
**Major violation of Dependency Injection**: The `Digest` struct in `digest/digest.go:41-55` internally creates all its dependencies (`hn.NewClient`, `scraper.NewScraper`, `summarizer.NewSummarizer`, `ranker.NewRanker`) rather than accepting interfaces. This makes the digest package impossible to unit test with mocks.

**No interfaces defined**: The spec explicitly requires narrow interfaces for HN client, scraper, summarizer, storage, and article sender. None exist - all components depend on concrete types.

Single responsibility is mostly followed within each package.

### 6. YAGNI Principle: 9/10
The implementation is focused and minimal. No speculative features or over-engineering. Minor issues:
- `RoundToDecimal` function defined in storage.go:388 but never used
- `FormatStartMessage` in bot.go:35 defined but never called

### 7. Error Handling & Resilience: 7/10
Graceful degradation is properly implemented:
- Scraper failure: Falls back to article title (digest.go:157-159) ✓
- Gemini failure: Skips article, continues (digest.go:170-177) ✓
- Send/DB failures: Logs, doesn't crash (main.go:131-138) ✓
- Graceful shutdown: SIGINT/SIGTERM caught (main.go:86-93) ✓

Errors are propagated with context using `fmt.Errorf("...: %w", err)` pattern. Deduction because reaction handling errors can't be tested since the feature doesn't exist.

### 8. Interface Design & Testability: 3/10
**No interfaces are defined** for any of the external dependencies. The spec explicitly states: "Each component should depend on narrow interfaces, not concrete types." This is not followed. Key issues:
- `Digest` struct depends on concrete `*storage.Storage`, `*hn.Client`, `*scraper.Scraper`, etc.
- Command handlers in main.go depend on concrete `*tgbotapi.BotAPI`
- This makes the `digest` package impossible to unit test without making real HTTP calls

The result is the `digest` package has no tests at all.

### 9. Logging & Observability: 8/10
Uses Go's `slog` package with JSON output as specified (main.go:26-27). Key events are logged:
- Startup (main.go:56)
- Digest cycle start/completion (digest.go:58, 147)
- API failures (digest.go:93, 103, 158, 171)
- Settings changes would be logged
- Scheduler started (main.go:82)

Minor deduction: No logging of reaction events since they're not implemented. Log level not dynamically set based on config.

### 10. Specification Adherence: 5/10
**Verified design decisions:**
1. Single-user only ✓
2. Long polling (uses library's GetUpdatesChan) ✓
3. Pure Go SQLite (modernc.org/sqlite) ✓
4. Synchronous pipeline ✓
5. Tag decay on fetch ✓
6. Direct HTTP for Gemini (no SDK) ✓
7. 70/30 blended ranking (ranker.go:53) ✓
8. Reaction idempotency - **NOT VERIFIABLE** (reaction handling missing)
9. 7-day recency filter ✓
10. Thread-safe settings - **MISSING** (no mutex in Digest)

**Missing per spec:**
- Reaction handling entirely missing
- Spec requires manual `getUpdates` calls for reaction support, but code uses library's built-in polling which may not support `message_reaction`
- Thread-safe settings via mutex not implemented

---

## Conclusion

This implementation has a solid foundation with proper package organization, good error handling, and structured logging. The basic bot commands work and the digest workflow is implemented correctly.

However, there are two critical failures: **(1) reaction handling is completely missing**, which means the core feature of learning user preferences through thumbs-up reactions does not work, and **(2) no interfaces are defined**, violating the spec's explicit requirement for dependency injection and making the critical `digest` package untestable. The 58.6% test coverage falls well short of the "near 100%" target, primarily because the central orchestration logic cannot be tested without interfaces. These issues significantly impact the codebase's functionality and maintainability.
