package main

import "github.com/gin-gonic/gin"

type Tier int

const (
	Cold Tier = iota // 0
	Warm             // 1
	Hot              // 2
)

func (t Tier) String() string {
	return [...]string{"COLD", "WARM", "HOT"}[t]
}

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
	Tier   Tier    `json:"tier"`
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
