package bot

import (
	"fmt"
	"html"
	"strings"
)

type ReactionData struct {
	Type  ReactionType `json:"type"`
	Emoji string       `json:"emoji"`
}

type ReactionType struct {
	Emoji string `json:"emoji"`
}

func EscapeHTML(text string) string {
	return html.EscapeString(text)
}

func FormatArticleMessage(icon, title, summary string, score, comments int, articleURL string, articleID int64) string {
	escapedTitle := EscapeHTML(title)
	escapedSummary := EscapeHTML(summary)

	return fmt.Sprintf(`<b>%s %s</b>
<i>%s</i>

📊 %d points | 💬 %d comments
🔗 <a href="%s">Read article</a>
💬 <a href="https://news.ycombinator.com/item?id=%d">Discussion</a>`,
		icon, escapedTitle, escapedSummary, score, comments, articleURL, articleID)
}

func FormatStartMessage() string {
	return `Welcome! I'm your Hacker News digest bot.

Commands:
• /fetch - Get articles now
• /settings - Configure digest time and count
• /stats - View your preferences

I'll learn your interests from the 👍 reactions you give!`
}

func FormatWelcomeMessage() string {
	return `👋 Welcome! Your chat ID has been saved.

Use /fetch to get articles immediately, or wait for your daily digest.`
}

func FormatSettingsDisplay(digestTime string, articleCount int) string {
	return fmt.Sprintf(`📅 <b>Digest Time:</b> %s
📰 <b>Articles per digest:</b> %d

Use /settings time HH:MM or /settings count N to change.`,
		digestTime, articleCount)
}

func FormatSettingsUpdate(setting, value string) string {
	if setting == "time" {
		return fmt.Sprintf("✅ Digest time updated to %s", value)
	}
	return fmt.Sprintf("✅ Article count updated to %s", value)
}

func FormatStats(tags []struct {
	Name   string
	Weight float64
}, likeCount int) string {
	if len(tags) == 0 || likeCount == 0 {
		return `📊 No preferences yet!

Tap the 👍 thumbs-up on articles you like, and I'll learn your interests!
Use /stats to see your preferences after you've liked some articles.`
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`📊 <b>Your Interests</b> (%d likes)

`, likeCount))

	for i, tag := range tags {
		if i >= 10 {
			break
		}
		sb.WriteString(fmt.Sprintf("• %s: %.2f\n", tag.Name, tag.Weight))
	}

	return sb.String()
}

func ParseCommand(text string) (command, args string) {
	text = strings.TrimSpace(text)

	if !strings.HasPrefix(text, "/") {
		return "", ""
	}

	parts := strings.SplitN(text, " ", 2)
	commandPart := parts[0]

	commandPart = strings.TrimPrefix(commandPart, "/")

	if idx := strings.Index(commandPart, "@"); idx != -1 {
		commandPart = commandPart[:idx]
	}

	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	return commandPart, args
}

func ParseSettingsArgs(input string) (key, value string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("empty input")
	}

	parts := strings.SplitN(input, " ", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format, expected 'key value'")
	}

	key = strings.TrimSpace(parts[0])
	value = strings.TrimSpace(parts[1])

	if key != "time" && key != "count" {
		return "", "", fmt.Errorf("invalid setting key: %s", key)
	}

	if key == "time" && !isValidTime(value) {
		return "", "", fmt.Errorf("invalid time format: %s", value)
	}

	return key, value, nil
}

func isValidTime(t string) bool {
	if len(t) != 5 || t[2] != ':' {
		return false
	}

	var hour, minute int
	_, err := fmt.Sscanf(t, "%d:%d", &hour, &minute)
	if err != nil {
		return false
	}

	if hour < 0 || hour > 23 {
		return false
	}
	if minute < 0 || minute > 59 {
		return false
	}

	return true
}

func IsThumbsUpReaction(reaction *ReactionData) bool {
	if reaction == nil {
		return false
	}
	return reaction.Type.Emoji == "👍" || reaction.Emoji == "👍"
}
