package main

import (
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func proxyGET(c *gin.Context, targetURL string) {
    resp, err := http.Get(targetURL)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func main() {

    r := gin.Default()

    analyticsURL := os.Getenv("ANALYTICS_URL")
    if analyticsURL == "" {
        analyticsURL = "http://localhost:8083"
    }

    // -------------------------------
    // Healthcheck
    // -------------------------------
    r.GET("/healthz", func(c *gin.Context) {
        c.JSON(200, gin.H{"ok": true})
    })

    // -------------------------------
    // Dummy news
    // -------------------------------
    r.GET("/news", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "use /analytics endpoints"})
    })

    // -------------------------------
    // /analytics/sentiment
    // -------------------------------
    r.GET("/analytics/sentiment", func(c *gin.Context) {
        proxyGET(c, analyticsURL+"/sentiment")
    })

    // -------------------------------
    // /analytics/top-keywords
    // -------------------------------
    r.GET("/analytics/top-keywords", func(c *gin.Context) {
        proxyGET(c, analyticsURL+"/top-keywords")
    })

    // -------------------------------
    // /analytics/trending-sentiment
    // -------------------------------
    r.GET("/analytics/trending-sentiment", func(c *gin.Context) {
        proxyGET(c, analyticsURL+"/trending-sentiment")
    })

    // -------------------------------
    // /news/recommend → no article ID
    // -------------------------------
    r.GET("/news/recommend", func(c *gin.Context) {
        limit := c.Query("limit") // optional
        url := analyticsURL + "/recommendations"

        if limit != "" {
            url += "?limit=" + limit
        }

        proxyGET(c, url)
    })


    r.Run(":8080")
}
