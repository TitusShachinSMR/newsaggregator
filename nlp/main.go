package main
}


func main(){
dsn := os.Getenv("DB_DSN")
if dsn=="" { dsn="postgres://newsuser:newspwd@localhost:5432/newsdb?sslmode=disable" }
db, err := sql.Open("postgres", dsn)
if err!=nil { log.Fatal(err) }
defer db.Close()


raddr := os.Getenv("REDIS_ADDR")
if raddr=="" { raddr="localhost:6379" }
rdb := redis.NewClient(&redis.Options{Addr:raddr})


for {
// BLPOP with timeout
res, err := rdb.BLPop(ctx, 5*time.Second, "nlp:queue").Result()
if err == redis.Nil || len(res)==0 { time.Sleep(1*time.Second); continue }
if err!=nil { log.Println("redis error", err); time.Sleep(2*time.Second); continue }
id := res[1]


var content string
var articleID int
q := `SELECT id, content FROM articles WHERE id=$1`
err = db.QueryRow(q, id).Scan(&articleID, &content)
if err!=nil { log.Println("db read", err); continue }


score := sentimentScore(content)
summ := summarize(content)
keywords := fmt.Sprintf("{\"starter\",\"example\"}")


_, err = db.Exec(`INSERT INTO nlp_results(article_id, summary, sentiment, keywords) VALUES($1,$2,$3,$4)`, articleID, summ, score, pqStringArrayFromJSON(keywords))
if err!=nil { log.Println("db write", err); continue }
log.Printf("processed article %d sentiment=%.2f", articleID, score)
}
}


// helper: convert simple JSON/text keywords into a Postgres TEXT[] -- for starter purpose we use a placeholder
func pqStringArrayFromJSON(js string) interface{}{
// In real code use github.com/lib/pq and pass pq.Array([]string{...})
return js
}