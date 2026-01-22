package scraper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScraper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
<html>
<head><title>Test Title</title></head>
<body>
<article>
<h1>Main Heading</h1>
<p>This is the main content of the article. It should be extracted by readability.</p>
</article>
</body>
</html>
`))
	}))
	defer server.Close()

	s := NewScraper(10)
	content, err := s.Scrape(server.URL)
	require.NoError(t, err)
	assert.Contains(t, content, "This is the main content")
}
