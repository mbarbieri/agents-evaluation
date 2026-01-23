package bot

import (
	"fmt"
	"html"
)

type ArticleMessage struct {
	ID       int
	Title    string
	URL      string
	Summary  string
	Score    int
	Comments int
}

func FormatArticleHTML(a ArticleMessage) string {
	hnURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%d", a.ID)
	return fmt.Sprintf(
		"<b>\U0001F4F0 %s</b>\n<i>%s</i>\n\n\U0001F4AF %d  \U0001F4AC %d\n<a href=\"%s\">Read</a> | <a href=\"%s\">Discuss</a>",
		html.EscapeString(a.Title),
		html.EscapeString(a.Summary),
		a.Score,
		a.Comments,
		a.URL,
		hnURL,
	)
}
