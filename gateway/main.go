package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)


func main(){
r := gin.Default()


analyticsURL := os.Getenv("ANALYTICS_URL")
if analyticsURL == "" { analyticsURL = "http://localhost:8083" }


r.GET("/healthz", func(c *gin.Context){ c.JSON(200, gin.H{"ok":true}) })


r.GET("/news", func(c *gin.Context){
// simple: query DB directly or call other services. We'll just return a placeholder.
c.JSON(200, gin.H{"message":"use /fetcher to store and /analytics to query"})
})


r.GET("/analytics/sentiment", func(c *gin.Context){
resp, err := http.Get(analyticsURL + "/sentiment")
if err != nil { c.JSON(500, gin.H{"error":err.Error()}); return }
defer resp.Body.Close()
c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
})


r.Run(":8080")
}