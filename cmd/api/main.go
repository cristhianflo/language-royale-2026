package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Signals struct {
	LprHits24h        int     `json:"lpr_hits_24h" binding:"required,min=0,max=10000"`
	AddressMatch      bool    `json:"address_match" binding:"required"`
	DistanceMiles     float64 `json:"distance_miles" binding:"required,min=0,max=10000"`
	DaysSinceLastSeen int     `json:"days_since_last_seen" binding:"required,min=0,max=36500"`
}

type CaseRequest struct {
	CaseID  string  `json:"case_id" binding:"required"`
	Signals Signals `json:"signals" binding:"required"`
}

type CaseResponse struct {
	CaseID string  `json:"case_id"`
	Score  float64 `json:"score"`
	Tier   string  `json:"tier"`
}

func getTier(score float64) string {
	if score >= 0.80 {
		return "HOT"
	}
	if score >= 0.50 {
		return "WARM"
	}
	return "COLD"
}

func main() {
	router := gin.Default()

	router.POST("/score", func(c *gin.Context) {
		var req CaseRequest

		// 1. Bind and Validate (Handles required keys and ranges)
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 2. Explicit check for trimmed length
		if len(strings.TrimSpace(req.CaseID)) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "case_id cannot be empty or whitespace"})
			return
		}

		// 3. Scoring Logic
		// (Mocking a score here; you can replace this with your sigmoid logic)
		score := 0.87

		// 4. Build Response
		resp := CaseResponse{
			CaseID: req.CaseID,
			Score:  score,
			Tier:   getTier(score),
		}

		// 5. Return 200 OK (Extra keys in request are ignored by default)
		c.JSON(http.StatusOK, resp)
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"ok": true,
		})
	})
	router.Run() // listens on 0.0.0.0:8080 by default
}
