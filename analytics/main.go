package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

// ----------------------------------------------------
// SMALL JSON HELPER
// ----------------------------------------------------
func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func main() {

	dsn := os.Getenv("DB_DSN")
		if dsn == "" {
    dsn = "postgres://newsuser:newspwd@postgres:5432/newsdb?sslmode=disable"
}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// ============================================================
	// 1️⃣  /sentiment   → Count positive / neutral / negative
	// ============================================================
	http.HandleFunc("/sentiment", func(w http.ResponseWriter, r *http.Request) {

		rows, err := db.Query(`
			SELECT sentiment, COUNT(*) 
			FROM nlp_results 
			GROUP BY sentiment 
			ORDER BY sentiment DESC
		`)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		type Bucket struct {
			Sentiment float64 `json:"sentiment"`
			Count     int     `json:"count"`
		}

		var result []Bucket
		for rows.Next() {
			var b Bucket
			rows.Scan(&b.Sentiment, &b.Count)
			result = append(result, b)
		}

		jsonResponse(w, result)
	})

	// ============================================================
	// 2️⃣  /top-keywords  → Finds most used keywords
	// ============================================================
	http.HandleFunc("/top-keywords", func(w http.ResponseWriter, r *http.Request) {

		rows, err := db.Query(`
			SELECT unnest(keywords) AS kw, COUNT(*)
			FROM nlp_results
			GROUP BY kw
			ORDER BY COUNT(*) DESC
			LIMIT 20
		`)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		type KW struct {
			Keyword string `json:"keyword"`
			Count   int    `json:"count"`
		}

		var list []KW

		for rows.Next() {
			var k KW
			rows.Scan(&k.Keyword, &k.Count)
			list = append(list, k)
		}

		jsonResponse(w, list)
	})

	// ============================================================
	// 3️⃣  /trending-sentiment → sentiment over time (per day)
	// ============================================================
	http.HandleFunc("/trending-sentiment", func(w http.ResponseWriter, r *http.Request) {

		rows, err := db.Query(`
			SELECT DATE(processed_at), AVG(sentiment)
			FROM nlp_results
			GROUP BY DATE(processed_at)
			ORDER BY DATE(processed_at)
			LIMIT 30
		`)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		type Trend struct {
			Date      string  `json:"date"`
			AvgSent   float64 `json:"avg_sentiment"`
		}

		var trend []Trend

		for rows.Next() {
			var t Trend
			rows.Scan(&t.Date, &t.AvgSent)
			trend = append(trend, t)
		}

		jsonResponse(w, trend)
	})
    // ============================================================
// 4️⃣  /recommendations → Suggest top news articles
// ============================================================
http.HandleFunc("/recommendations", func(w http.ResponseWriter, r *http.Request) {

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			limit = n
		}
	}

	// Ranking Logic:
	// - higher sentiment → more positive
	// - longer summary → more info
	// - newer articles preferred
	rows, err := db.Query(`
		SELECT a.id, a.title, a.url, a.published_at, 
		       n.summary, n.sentiment, n.keywords
		FROM articles a
		INNER JOIN nlp_results n ON a.id = n.article_id
		ORDER BY 
			n.sentiment DESC,           -- positive first
			a.published_at DESC,        -- newer first
			length(n.summary) DESC      -- rich summaries
		LIMIT $1
	`, limit)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	type Rec struct {
		ID        int      `json:"id"`
		Title     string   `json:"title"`
		URL       string   `json:"url"`
		Date      string   `json:"date"`
		Summary   string   `json:"summary"`
		Sentiment float64  `json:"sentiment"`
		Keywords  []string `json:"keywords"`
	}

	var recs []Rec

	for rows.Next() {
		var r Rec
		var kw []string

		rows.Scan(
			&r.ID, &r.Title, &r.URL, &r.Date,
			&r.Summary, &r.Sentiment, pq.Array(&kw),
		)
		r.Keywords = kw
		recs = append(recs, r)
	}

	jsonResponse(w, recs)
})

	// ============================================================
	// START SERVICE
	// ============================================================
	log.Println("analytics listening :8083")
	http.ListenAndServe(":8083", nil)
}
