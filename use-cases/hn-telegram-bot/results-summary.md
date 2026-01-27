# AI Agent Evaluation Summary: HN Telegram Bot

## Overall Ranking

| Rank | Agent | Total Score |
|------|-------|-------------|
| 1st | **claude-code-opus-4.5** | **92/100** |
| 2nd | claude-code-sonnet-4.5 | 82/100 |
| 3rd | opencode-gpt-5.2-codex | 79/100 |
| 4th | opencode-gpt-5.2 | 78/100 |
| 5th | opencode-gemini-3-pro-high | 76/100 |
| 6th | opencode-gemini-3-flash | 71/100 |
| 7th | opencode-kimi-2.5 | 65/100 |
| 8th | opencode-minimax-m2.1 | 61/100 |
| 9th | opencode-glm-4.7 | 56/100 |
| 10th | opencode-grok-code-fast-1 | 49/100 |

---

## Resource Usage

| Agent | Time Taken | Tokens Used | Lines of Code (Go) |
|-------|------------|-------------|-------------------|
| claude-code-opus-4.5 | ~13 min | 105k | 3,855 |
| claude-code-sonnet-4.5 | ~17 min | 114k | 3,305 |
| opencode-gpt-5.2-codex | N/A | N/A | 3,166 |
| opencode-gpt-5.2 | ~24 min | 84k | 2,995 |
| opencode-gemini-3-pro-high | ~30 min | 73k | 1,853 |
| opencode-gemini-3-flash | ~10 min | 65k | 1,485 |
| opencode-kimi-2.5 | ~20 min | 119k | 4,084 |
| opencode-minimax-m2.1 | ~20 min | 110k | 3,063 |
| opencode-glm-4.7 | ~2 hours | 97k | 3,229 |
| opencode-grok-code-fast-1 | ~15 min | 53k | 1,546 |

---

## Category Winners & Losers

| Category | Winner | Score | Loser | Score |
|----------|--------|-------|-------|-------|
| Functional Completeness | claude-code-opus-4.5 | 10 | opencode-grok-code-fast-1 | 3 |
| Test Quality & Coverage | claude-code-opus-4.5 | 9 | opencode-gemini-3-flash | 4 |
| Code Clarity | claude-code-opus-4.5 | 9 | gemini-flash/glm/grok (tie) | 7 |
| DRY Principle | opus-4.5/gemini-pro/kimi-2.5 (tie) | 8 | opencode-glm-4.7 | 6 |
| SOLID Principles | opus/sonnet/gpt-5.2-codex/gemini-pro (tie) | 9 | opencode-minimax-m2.1 | 4 |
| YAGNI Principle | claude-code-opus-4.5 | 10 | flash/glm/grok (tie) | 8 |
| Error Handling | claude-code-opus-4.5 | 9 | opencode-grok-code-fast-1 | 4 |
| Interface Design | opus/sonnet/gpt-5.2-codex/gemini-pro (tie) | 9 | opencode-minimax-m2.1 | 3 |
| Logging & Observability | claude-code-opus-4.5 | 9 | opencode-grok-code-fast-1 | 1 |
| Specification Adherence | claude-code-opus-4.5 | 10 | opencode-grok-code-fast-1 | 3 |

---

## Full Comparison Table

| Category | opus-4.5 | sonnet-4.5 | gpt-5.2-codex | gpt-5.2 | gemini-pro | gemini-flash | kimi-2.5 | minimax-m2.1 | glm-4.7 | grok-fast-1 |
|----------|----------|------------|---------------|---------|------------|--------------|----------|--------------|---------|-------------|
| Functional Completeness | **10** | 8 | 9 | 9 | 8 | 8 | 5 | 5 | 4 | 3 |
| Test Quality & Coverage | **9** | 7 | 5 | 5 | 6 | 4 | 5 | 5 | 5 | 5 |
| Code Clarity | **9** | 8 | 8 | 8 | 8 | 7 | 8 | 8 | 7 | 7 |
| DRY Principle | **8** | 7 | 7 | 7 | **8** | 7 | **8** | 7 | 6 | 7 |
| SOLID Principles | **9** | **9** | **9** | 8 | **9** | 7 | 6 | 4 | 6 | 6 |
| YAGNI Principle | **10** | 9 | 9 | 9 | 9 | 8 | 9 | 9 | 8 | 8 |
| Error Handling | **9** | 8 | 8 | 8 | 7 | 8 | 7 | 7 | 5 | 4 |
| Interface Design | **9** | **9** | **9** | 8 | **9** | 7 | 5 | 3 | 5 | 5 |
| Logging & Observability | **9** | 8 | 6 | 7 | 5 | 7 | 7 | 8 | 6 | 1 |
| Specification Adherence | **10** | 9 | 9 | 9 | 7 | 8 | 5 | 5 | 4 | 3 |
| **TOTAL** | **92** | **82** | **79** | **78** | **76** | **71** | **65** | **61** | **56** | **49** |

---

## Key Findings

**claude-code-opus-4.5** dominated across all categories, winning or tying for first in every single category. It was the only agent to produce fully functional code meeting all specification requirements.

**claude-code-sonnet-4.5** placed second with 82/100, tying for first in SOLID Principles and Interface Design. Main weaknesses were test coverage (bot package at 51%) and some DRY violations (duplicated validation regex, hardcoded defaults). Minor spec deviation: `/fetch` shows confirmation message before running digest.

**opencode-gpt-5.2-codex** placed third with 79/100, demonstrating excellent interface-driven design with the adapter pattern cleanly separating infrastructure from business logic. Tied for first in SOLID Principles and Interface Design (9/10 each). All major features implemented correctly with proper graceful degradation. Main weakness was test coverage (19-86% range, averaging ~60%), falling short of the "near 100%" target. Logging was incomplete (missing startup, digest stage, and success event logs). All 10 intentional design decisions preserved including thread-safe settings via sync.RWMutex.

**opencode-gpt-5.2** placed fourth with 78/100, demonstrating solid architectural understanding with proper interface segregation and dependency injection. All specified features work correctly including the digest pipeline, reaction handling, and preference learning. Main weakness was test coverage (~65% average vs "near 100%" required), with the bot package critically undertested at 4.1%. Thread-safe settings properly implemented with sync.RWMutex. All 10 intentional design decisions preserved.

**opencode-gemini-3-pro-high** was fifth, tying for first in 3 categories (DRY, SOLID, Interface Design) but struggled with logging consistency.

**opencode-kimi-2.5** placed seventh with 65/100. The implementation has well-organized code with clear naming (8/10 clarity, 8/10 DRY, 9/10 YAGNI), but suffers from critical functional gaps. Reaction handling is completely broken (`message_reaction` never requested from Telegram API), the 70/30 ranking formula has a bug (passes wrong parameters), and comment counts are always zero. Test coverage is highly uneven: ranker/scheduler at 100%, but bot at 7.6% and digest at 6.4%—the core packages were left essentially untested. Thread-safe settings properly implemented with mutex.

**opencode-grok-code-fast-1** had the most critical failures: no main.go entry point (application cannot run), empty digest package, and only 1 of 4 bot commands implemented. Logging was essentially absent (1/10).

**Common weaknesses across agents:**
- Test coverage below the "near 100%" spec requirement
- Missing thread-safe settings (mutex protection) - except opus-4.5, gpt-5.2-codex, gpt-5.2, and kimi-2.5
- Reaction handling not properly wired in many implementations
- Inconsistent use of slog structured logging
