package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/mmcdole/gofeed"
)

var ctx = context.Background()

// List of RSS Feeds
var rssFeeds = []string{
	"https://feeds.bbci.co.uk/news/rss.xml",
	"https://rss.cnn.com/rss/edition.rss",
	"https://feeds.skynews.com/feeds/rss/home.xml",
}

func main() {

	// ------------------ CONNECT POSTGRES ------------------
	dsn := os.Getenv("DB_DSN")
		if dsn == "" {
    dsn = "postgres://newsuser:newspwd@postgres:5432/newsdb?sslmode=disable"
}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Postgres connect error:", err)
	}
    // Wait until Postgres is ready
for {
    err := db.Ping()
    if err == nil {
        log.Println("Connected to Postgres!")
        break
    }
    log.Println("Postgres not ready, retrying in 2s...")
    time.Sleep(2 * time.Second)
}

	// ------------------ CONNECT REDIS ------------------
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	// ------------------ RSS PARSER ------------------
	parser := gofeed.NewParser()

	log.Println("Fetcher service (RSS) started...")

	for {
		for _, feedURL := range rssFeeds {

			feed, err := parser.ParseURL(feedURL)
			if err != nil {
				log.Println("RSS error:", err)
				continue
			}

			for _, item := range feed.Items {

				if item.Link == "" {
					continue
				}

				// check duplicate by URL
				var exists bool
				err := db.QueryRow(
					`SELECT EXISTS(SELECT 1 FROM articles WHERE url=$1)`,
					item.Link,
				).Scan(&exists)

				if err != nil {
					log.Println("Dup check error:", err)
					continue
				}

				if exists {
					continue // skip duplicates
				}

				// Insert article
				var id int
				err = db.QueryRow(`
					INSERT INTO articles (source, title, content, url, published_at)
					VALUES ($1, $2, $3, $4, $5)
					RETURNING id
				`,
					feed.Title,
					item.Title,
					item.Content,
					item.Link,
					item.PublishedParsed,
				).Scan(&id)

				if err != nil {
					log.Println("DB insert error:", err)
					continue
				}

				// Push to Redis NLP queue
				_, err = rdb.LPush(ctx, "nlp:queue", id).Result()
				if err != nil {
					log.Println("Redis push error:", err)
					continue
				}

				log.Printf("Inserted & queued article id=%d (%s)\n", id, item.Title)
			}
		}

		// poll RSS every 10 minutes
		time.Sleep(10 * time.Minute)
	}
}
