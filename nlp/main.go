package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

var ctx = context.Background()

// ---- HF API helpers ----

type HFRequest struct {
    Inputs string `json:"inputs"`
}

type HFTextResult struct {
    SummaryText string `json:"summary_text"`
}

type HFSentimentResult struct {
    Label string  `json:"label"`
    Score float64 `json:"score"`
}

func hfRequest(model, text string, out interface{}) error {
    apiKey := os.Getenv("HF_API_KEY")
    if apiKey == "" {
        return fmt.Errorf("HF_API_KEY not set")
    }

    url := fmt.Sprintf("https://api-inference.huggingface.co/models/%s", model)

    payload := HFRequest{Inputs: text}
    body, _ := json.Marshal(payload)

    req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()

    respBody, _ := ioutil.ReadAll(resp.Body)
    return json.Unmarshal(respBody, out)
}

func summarize(text string) (string, error) {
    var result []HFTextResult
    err := hfRequest("facebook/bart-large-cnn", text, &result)
    if err != nil { return "", err }

    if len(result) > 0 {
        return result[0].SummaryText, nil
    }
    return "", fmt.Errorf("no summary returned")
}

func sentiment(text string) (float64, error) {
    var result []HFSentimentResult
    err := hfRequest("distilbert-base-uncased-finetuned-sst-2-english", text, &result)
    if err != nil { return 0, err }

    if len(result) > 0 {
        if result[0].Label == "POSITIVE" {
            return result[0].Score, nil
        }
        return -result[0].Score, nil
    }
    return 0, fmt.Errorf("no sentiment returned")
}

// ---- Main worker ----

func main() {
    dsn := os.Getenv("DB_DSN")
    if dsn == "" { dsn = "postgres://newsuser:newspwd@localhost:5432/newsdb?sslmode=disable" }
    db, err := sql.Open("postgres", dsn)
    if err != nil { log.Fatal(err) }
    defer db.Close()

    raddr := os.Getenv("REDIS_ADDR")
    if raddr == "" { raddr = "localhost:6379" }
    rdb := redis.NewClient(&redis.Options{Addr: raddr})

    log.Println("NLP service (HuggingFace version) running...")

    for {
        res, err := rdb.BLPop(ctx, 5*time.Second, "nlp:queue").Result()
        if err == redis.Nil || len(res) == 0 {
            time.Sleep(1 * time.Second)
            continue
        }
        if err != nil {
            log.Println("redis error:", err)
            continue
        }

        articleID := res[1]

        var content string
        q := `SELECT content FROM articles WHERE id=$1`
        err = db.QueryRow(q, articleID).Scan(&content)
        if err != nil {
            log.Println("db read error:", err)
            continue
        }

        // ---- HF summary & sentiment ----
        summ, err := summarize(content)
        if err != nil { log.Println("summary error:", err); continue }

        score, err := sentiment(content)
        if err != nil { log.Println("sentiment error:", err); continue }

        _, err = db.Exec(`
            INSERT INTO nlp_results(article_id, summary, sentiment, keywords)
            VALUES ($1, $2, $3, $4)
        `, articleID, summ, score, pq.Array([]string{"news", "analysis"}))
        if err != nil {
            log.Println("db insert:", err)
            continue
        }

        log.Printf("Processed article %s (sentiment=%.3f)", articleID, score)
    }
}
