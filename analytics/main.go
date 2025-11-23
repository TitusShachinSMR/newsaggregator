package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)


func main(){
dsn := os.Getenv("DB_DSN")
if dsn=="" { dsn="postgres://newsuser:newspwd@localhost:5432/newsdb?sslmode=disable" }
db, err := sql.Open("postgres", dsn)
if err!=nil { log.Fatal(err) }
defer db.Close()


http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.Write([]byte("ok")) })


http.HandleFunc("/sentiment", func(w http.ResponseWriter, r *http.Request){
rows, err := db.Query(`SELECT sentiment, count(*) FROM nlp_results GROUP BY sentiment ORDER BY sentiment DESC LIMIT 50`)
if err!=nil { http.Error(w, err.Error(), 500); return }
defer rows.Close()
w.Header().Set("Content-Type","application/json")
w.Write([]byte(`{"buckets":[]}`))
// starter returns empty JSON; extend with aggregation pipeline
})


log.Println("analytics listening :8083")
http.ListenAndServe(":8083", nil)
}