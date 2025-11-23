package shared

type Article struct {
	ID          int    `json:"id"`
	Source      string `json:"source"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at"`
}

type NLPResult struct {
	ArticleID int      `json:"article_id"`
	Summary   string   `json:"summary"`
	Sentiment float64  `json:"sentiment"`
	Keywords  []string `json:"keywords"`
}