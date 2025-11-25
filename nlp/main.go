package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hupe1980/go-huggingface"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

var ctx = context.Background()

// ===============================================================
// SUMMARY USING OFFICIAL HF Summarization API
// ===============================================================
func summarize(client *huggingface.InferenceClient, text string) (string, error) {

	// Must wrap text inside []string
	req := &huggingface.SummarizationRequest{
		Inputs: []string{text},
		Model:  "facebook/bart-large-cnn", // this is important
		Parameters: huggingface.SummarizationParameters{
			MinLength:        huggingface.PTR(30),
			MaxLength:        huggingface.PTR(150),
			Temperature:      huggingface.PTR(0.3),
			RepetitionPenalty: huggingface.PTR(1.1),
		},
	}

	resp, err := client.Summarization(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp) == 0 {
		return "", fmt.Errorf("empty summarization response")
	}

	return strings.TrimSpace(resp[0].SummaryText), nil
}

// ===============================================================
// SENTIMENT USING TextClassification API
// ===============================================================
func sentiment(client *huggingface.InferenceClient, text string) (float64, error) {

	req := &huggingface.TextClassificationRequest{
		Inputs: text,
		Model:  "distilbert-base-uncased-finetuned-sst-2-english",
	}

	resp, err := client.TextClassification(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("sentiment error: %w", err)
	}

	if len(resp) == 0 || len(resp[0]) == 0 {
		return 0, fmt.Errorf("sentiment empty result")
	}

	item := resp[0][0]
	label := strings.ToLower(item.Label)
	score := float64(item.Score)

	switch label {
	case "positive":
		return score, nil
	case "negative":
		return -score, nil
	default:
		return 0, nil // neutral
	}
}

// ===============================================================
// MAIN WORKER LOOP
// ===============================================================
func main() {

	// ---------------------- PostgreSQL ----------------------
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://newsuser:newspwd@postgres:5432/newsdb?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("DB open error:", err)
	}
	defer db.Close()

	// Wait for Postgres
	for {
		if err := db.Ping(); err == nil {
			break
		}
		log.Println("Waiting for Postgres...")
		time.Sleep(2 * time.Second)
	}

	// ---------------------- Redis ---------------------------
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// ---------------------- HuggingFace Client ----------------
	hfToken := os.Getenv("HF_API_KEY")
	if hfToken == "" {
		log.Fatal("ERROR: HF_API_KEY not set")
	}

	client := huggingface.NewInferenceClient(hfToken)

	log.Println("NLP service (HuggingFace official SDK) ready...")

	// ---------------------- Worker Loop ---------------------
	for {
		res, err := rdb.BLPop(ctx, 5*time.Second, "nlp:queue").Result()
		if err == redis.Nil || len(res) < 2 {
			continue
		}
		if err != nil {
			log.Println("Redis error:", err)
			continue
		}

		articleID := res[1]

		var content string
		err = db.QueryRow(`SELECT content FROM articles WHERE id=$1`, articleID).Scan(&content)
		if err != nil {
			log.Println("DB read error:", err)
			continue
		}

		// ------------------ Summary ------------------
		summary, err := summarize(client, content)
		if err != nil {
			log.Println("Summary error:", err)
			continue
		}

		// ------------------ Sentiment -----------------
		score, err := sentiment(client, content)
		if err != nil {
			log.Println("Sentiment error:", err)
			continue
		}

		// ------------------ Insert Output -------------
		_, err = db.Exec(`
			INSERT INTO nlp_results(article_id, summary, sentiment, keywords)
			VALUES ($1, $2, $3, $4)
		`, articleID, summary, score, pq.Array([]string{"news", "analysis"}))

		if err != nil {
			log.Println("DB insert error:", err)
			continue
		}

		log.Printf("✔ Processed article %s (sentiment %.3f)\n", articleID, score)
	}
}
