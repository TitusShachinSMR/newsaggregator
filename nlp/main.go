package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
)

var ctx = context.Background()

// ------------------------
// OLLAMA GENERATE REQUEST
// ------------------------

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func callOllama(model, prompt string) (string, error) {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://ollama:11434"
	}

	url := host + "/api/generate"

	body, _ := json.Marshal(OllamaRequest{
		Model:  model,
		Prompt: prompt,
	})

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var parsed OllamaResponse
	if err := json.Unmarshal(raw, &parsed); err == nil {
		return strings.TrimSpace(parsed.Response), nil
	}

	return "", fmt.Errorf("ollama unexpected response: %s", string(raw))
}

// ------------------------
// SUMMARY
// ------------------------

func summarize(text string) (string, error) {
	prompt := "Summarize this news article in 3–4 lines:\n\n" + text
	return callOllama("mistral", prompt)
}

// ------------------------
// SENTIMENT
// ------------------------

func sentiment(text string) (float64, error) {
	prompt := `
Classify sentiment of the text as:
1 (positive)
0 (neutral)
-1 (negative)

Text:
` + text

	resp, err := callOllama("mistral", prompt)
	if err != nil {
		return 0, err
	}

	resp = strings.ToLower(strings.TrimSpace(resp))

	switch resp {
	case "1", "+1", "positive":
		return 1, nil
	case "-1", "- 1", "negative":
		return -1, nil
	default:
		return 0, nil
	}
}

// ------------------------
// MAIN WORKER LOOP
// ------------------------

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://newsuser:newspwd@localhost:5432/newsdb?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})

	log.Println("NLP service (Ollama version) running...")

	for {
		res, err := rdb.BLPop(ctx, 5*time.Second, "nlp:queue").Result()
		if err == redis.Nil || len(res) < 2 {
			continue
		}
		if err != nil {
			log.Println("redis error:", err)
			continue
		}

		articleID := res[1]

		var content string
		err = db.QueryRow(`SELECT content FROM articles WHERE id=$1`, articleID).Scan(&content)
		if err != nil {
			log.Println("db error:", err)
			continue
		}

		summary, err := summarize(content)
		if err != nil {
			log.Println("summary error:", err)
			continue
		}

		score, err := sentiment(content)
		if err != nil {
			log.Println("sentiment error:", err)
			continue
		}

		_, err = db.Exec(`
            INSERT INTO nlp_results(article_id, summary, sentiment, keywords)
            VALUES ($1, $2, $3, $4)`,
			articleID, summary, score, pq.Array([]string{"news", "analysis"}))

		if err != nil {
			log.Println("db insert error:", err)
			continue
		}

		log.Printf("Processed article %s (sentiment=%.1f)", articleID, score)
	}
}
