package digest

import (
	"fmt"
	"html"
)

func FormatArticleMessage(article Article) string {
	title := html.EscapeString(article.Title)
	summary := html.EscapeString(article.Summary)
	url := article.URL
	commentURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%d", article.ID)

	return fmt.Sprintf(
		"📰 <b>%s</b>\n<i>%s</i>\n⭐ %d   💬 %d\n🔗 <a href=\"%s\">Article</a> | <a href=\"%s\">HN</a>",
		title,
		summary,
		article.Score,
		article.Comments,
		url,
		commentURL,
	)
}
