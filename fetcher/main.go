package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"context"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
)


var ctx = context.Background()


func main(){
dsn := os.Getenv("DB_DSN")
if dsn=="" { dsn="postgres://newsuser:newspwd@localhost:5432/newsdb?sslmode=disable" }
db, err := sql.Open("postgres", dsn)
if err!=nil { log.Fatal(err) }
defer db.Close()


raddr := os.Getenv("REDIS_ADDR")
if raddr=="" { raddr="localhost:6379" }
rdb := redis.NewClient(&redis.Options{Addr:raddr})


// Simple placeholder article - replace with NewsAPI call
title := "Starter: Example article " + time.Now().Format(time.RFC3339)
content := "This is a sample content fetched by fetcher service."
url := fmt.Sprintf("https://example.com/%d", time.Now().Unix())


var id int
err = db.QueryRow(`INSERT INTO articles(source,title,content,url,published_at) VALUES($1,$2,$3,$4,$5) RETURNING id`, "example", title, content, url, time.Now()).Scan(&id)
if err!=nil { log.Fatal(err) }


// push to redis queue for NLP service
err = rdb.RPush(ctx, "nlp:queue", id).Err()
if err!=nil { log.Fatal(err) }


log.Printf("Inserted article id=%d and queued for NLP", id)


// For demo we exit. In production run periodically (ticker) or as cron.
}