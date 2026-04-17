package main

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type CaseResponse struct {
	CaseID string  `json:"case_id"`
	Score  float64 `json:"score"`
	Tier   string  `json:"tier"`
}

type scoreInput struct {
	CaseID            string
	LprHits24h        int64
	AddressMatch      bool
	DistanceMiles     float64
	DaysSinceLastSeen int64
}

var errInvalidInput = errors.New("invalid input")

func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

func sigmoid(value float64) float64 {
	return 1 / (1 + math.Exp(-value))
}

func calculateScore(input scoreInput) float64 {
	addressMatch := 0.0
	if input.AddressMatch {
		addressMatch = 1.2
	}

	rawScore := 0.9*float64(input.LprHits24h) + addressMatch - 0.35*input.DistanceMiles - 0.6*float64(input.DaysSinceLastSeen)
	return roundToTwoDecimals(sigmoid(rawScore))
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

func parseScoreInput(reader io.Reader) (scoreInput, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return scoreInput{}, errInvalidInput
	}

	var extraData any
	if err := decoder.Decode(&extraData); err != io.EOF {
		return scoreInput{}, errInvalidInput
	}

	caseID, ok := payload["case_id"].(string)
	if !ok || len(strings.TrimSpace(caseID)) == 0 {
		return scoreInput{}, errInvalidInput
	}

	signals, ok := payload["signals"].(map[string]any)
	if !ok {
		return scoreInput{}, errInvalidInput
	}

	lprHits24h, err := parseRequiredInt(signals, "lpr_hits_24h", 0, 10000)
	if err != nil {
		return scoreInput{}, err
	}

	addressMatch, ok := signals["address_match"].(bool)
	if !ok {
		return scoreInput{}, errInvalidInput
	}

	distanceMiles, err := parseRequiredNumber(signals, "distance_miles", 0, 10000)
	if err != nil {
		return scoreInput{}, err
	}

	daysSinceLastSeen, err := parseRequiredInt(signals, "days_since_last_seen", 0, 36500)
	if err != nil {
		return scoreInput{}, err
	}

	return scoreInput{
		CaseID:            caseID,
		LprHits24h:        lprHits24h,
		AddressMatch:      addressMatch,
		DistanceMiles:     distanceMiles,
		DaysSinceLastSeen: daysSinceLastSeen,
	}, nil
}

func parseRequiredInt(values map[string]any, key string, min int64, max int64) (int64, error) {
	number, ok := values[key].(json.Number)
	if !ok {
		return 0, errInvalidInput
	}

	parsed, err := number.Int64()
	if err != nil || parsed < min || parsed > max {
		return 0, errInvalidInput
	}

	return parsed, nil
}

func parseRequiredNumber(values map[string]any, key string, min float64, max float64) (float64, error) {
	number, ok := values[key].(json.Number)
	if !ok {
		return 0, errInvalidInput
	}

	parsed, err := number.Float64()
	if err != nil || parsed < min || parsed > max {
		return 0, errInvalidInput
	}

	return parsed, nil
}

func newRouter() *gin.Engine {
	router := gin.Default()

	router.POST("/score", func(c *gin.Context) {
		input, err := parseScoreInput(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidInput.Error()})
			return
		}

		score := calculateScore(input)

		resp := CaseResponse{
			CaseID: input.CaseID,
			Score:  score,
			Tier:   getTier(score),
		}

		c.JSON(http.StatusOK, resp)
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ok": true,
		})
	})

	return router
}

func main() {
	router := newRouter()
	router.Run() // listens on 0.0.0.0:8080 by default
}
