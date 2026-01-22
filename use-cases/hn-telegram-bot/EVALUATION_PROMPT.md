# Codebase Quality Evaluation Prompt

You are a code quality evaluator. Your task is to assess the quality of a software implementation against its specification.

## Inputs

1. **Specification**: Read the `SPECIFICATION.md` file in this directory
2. **Source Code**: The implementation is located in: `opencode-minimax-m2.1`

## Instructions

1. Read the SPECIFICATION.md file thoroughly to understand what the software should do
2. Explore the codebase systematically:
   - Examine the project structure and package organization
   - Read source files and their corresponding test files
   - Analyze how components interact
3. Evaluate the implementation against each category below
4. Provide a rating and justification for each category
5. Produce a final summary report

## Evaluation Categories

### 1. Functional Completeness (0-10)

Does the implementation cover all specified features?

**Check for:**
- All 4 bot commands implemented: `/start`, `/fetch`, `/settings`, `/stats`
- Reaction handling with thumbs-up detection and preference boosting
- Daily digest workflow with all pipeline stages (decay, fetch, filter, scrape, summarize, rank, send)
- Preference learning algorithm (boost on like, decay on fetch cycle)
- All 9 packages exist: `config`, `storage`, `bot`, `hn`, `scraper`, `summarizer`, `ranker`, `scheduler`, `digest`
- Configuration loading with defaults, validation, and environment variable overrides
- Database schema with all 4 tables: articles, likes, tag_weights, settings
- Graceful shutdown handling SIGINT/SIGTERM

---

### 2. Test Quality & Coverage (0-10)

The specification mandates TDD and near 100% test coverage.

**Check for:**
- Each source file has a corresponding `_test.go` file
- Table-driven tests for validation and parameterized logic
- Mock implementations for all interfaces
- Tests cover both success paths and error cases
- Key areas tested: config, storage, commands, reactions, ranker, scheduler, summarizer, workflow

---

### 3. Code Clarity (0-10)

Self-explanatory code with clear naming.

**Check for:**
- Function and variable names clearly express intent
- Code structure is easy to follow
- Functions are appropriately sized
- Control flow is linear and predictable

---

### 4. DRY Principle (0-10)

No duplicate logic across the codebase.

**Check for:**
- Common patterns extracted into shared functions
- No copy-pasted code blocks
- Error handling patterns consistent

---

### 5. SOLID Principles (0-10)

Single responsibility, dependency injection, interface segregation.

**Check for:**
- Each package/type has one clear purpose
- Narrow, focused interfaces
- Components depend on interfaces, not concrete types
- Dependencies are injected, not created internally

---

### 6. YAGNI Principle (0-10)

No speculative features or over-engineering.

**Check for:**
- Only specified features implemented
- No unused code, types, or functions
- No premature abstractions
- Simplest solution that meets requirements

---

### 7. Error Handling & Resilience (0-10)

Graceful degradation as specified.

**Check for:**
- Scraper failure: Falls back to article title
- Gemini failure: Skips article, continues processing
- Send/DB failures: Logs, doesn't crash
- Graceful shutdown: Catches signals, cancels context, closes resources
- Errors propagated (not panics) with sufficient context

---

### 8. Interface Design & Testability (0-10)

Components designed for testing with dependency injection.

**Check for:**
- Interfaces defined for external dependencies
- Easy to create mock implementations
- No global state that complicates testing

---

### 9. Logging & Observability (0-10)

Structured logging with appropriate coverage.

**Check for:**
- Uses Go's `slog` package with JSON output
- Key events logged: startup, digest cycle stages, API failures, reactions, settings changes
- Appropriate log levels
- No sensitive data logged

---

### 10. Specification Adherence (0-10)

Preserves the 10 intentional design decisions from the spec.

**Verify:**
1. Single-user only
2. Long polling (not webhooks)
3. Pure Go SQLite (modernc.org/sqlite)
4. Synchronous pipeline
5. Tag decay on fetch (not time-based)
6. Direct HTTP for Gemini (no SDK)
7. 70/30 blended ranking formula
8. Reaction idempotency
9. 7-day recency filter
10. Thread-safe settings

---

## Output Format

Produce your evaluation as a structured report:

```markdown
# Codebase Evaluation Report

## Project: {project name}
## Date: {date}
## Code Location: {folder path}

---

## Scores

| Category | Score |
|----------|-------|
| Functional Completeness | X/10 |
| Test Quality & Coverage | X/10 |
| Code Clarity | X/10 |
| DRY Principle | X/10 |
| SOLID Principles | X/10 |
| YAGNI Principle | X/10 |
| Error Handling & Resilience | X/10 |
| Interface Design & Testability | X/10 |
| Logging & Observability | X/10 |
| Specification Adherence | X/10 |
| **Total** | **X/100** |

---

## Justifications

### 1. Functional Completeness: X/10
[Brief justification]

### 2. Test Quality & Coverage: X/10
[Brief justification]

### 3. Code Clarity: X/10
[Brief justification]

### 4. DRY Principle: X/10
[Brief justification]

### 5. SOLID Principles: X/10
[Brief justification]

### 6. YAGNI Principle: X/10
[Brief justification]

### 7. Error Handling & Resilience: X/10
[Brief justification]

### 8. Interface Design & Testability: X/10
[Brief justification]

### 9. Logging & Observability: X/10
[Brief justification]

### 10. Specification Adherence: X/10
[Brief justification]

---

## Conclusion

[1-2 paragraph summary of overall quality]
```

---

## Notes for the Evaluator

- Be objective and evidence-based
- Don't assume functionality works; verify by reading the implementation
- Weight functional correctness heavily; beautiful code that doesn't work is worthless
