package main

import "github.com/gin-gonic/gin"

type CaseSignals struct {
	LprHits24h        int     `json:"lpr_hits_24h"`
	AddressMatch      bool    `json:"address_match"`
	DistanceMiles     float64 `json:"distance_miles"`
	DaysSinceLastSeen int     `json:"days_since_last_seen"`
}

type CaseRequest struct {
	CaseID  string      `json:"case_id" binding:"required"`
	Signals CaseSignals `json:"signals"`
}

func main() {
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"ok": true,
		})
	})
	router.Run() // listens on 0.0.0.0:8080 by default
}
