package main

import (
	"bytes"
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

type rawPayload map[string]json.RawMessage

var errInvalidInput = errors.New("invalid input")
var jsonNull = []byte("null")

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

	var payload rawPayload
	if err := decoder.Decode(&payload); err != nil {
		return scoreInput{}, errInvalidInput
	}

	var extraData any
	if err := decoder.Decode(&extraData); err != io.EOF {
		return scoreInput{}, errInvalidInput
	}

	caseID, err := parseRequiredString(payload, "case_id")
	if err != nil {
		return scoreInput{}, errInvalidInput
	}

	signals, err := parseRequiredObject(payload, "signals")
	if err != nil {
		return scoreInput{}, errInvalidInput
	}

	lprHits24h, err := parseRequiredInt(signals, "lpr_hits_24h", 0, 10000)
	if err != nil {
		return scoreInput{}, err
	}

	addressMatch, err := parseRequiredBool(signals, "address_match")
	if err != nil {
		return scoreInput{}, err
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

func parseRequiredString(values rawPayload, key string) (string, error) {
	rawValue, ok := values[key]
	if !ok || isNullJSON(rawValue) {
		return "", errInvalidInput
	}

	var parsed string
	if err := json.Unmarshal(rawValue, &parsed); err != nil || len(strings.TrimSpace(parsed)) == 0 {
		return "", errInvalidInput
	}

	return parsed, nil
}

func parseRequiredObject(values rawPayload, key string) (rawPayload, error) {
	rawValue, ok := values[key]
	if !ok || isNullJSON(rawValue) {
		return nil, errInvalidInput
	}

	var parsed rawPayload
	if err := json.Unmarshal(rawValue, &parsed); err != nil {
		return nil, errInvalidInput
	}

	return parsed, nil
}

func parseRequiredBool(values rawPayload, key string) (bool, error) {
	rawValue, ok := values[key]
	if !ok || isNullJSON(rawValue) {
		return false, errInvalidInput
	}

	var parsed bool
	if err := json.Unmarshal(rawValue, &parsed); err != nil {
		return false, errInvalidInput
	}

	return parsed, nil
}

func parseRequiredInt(values rawPayload, key string, min int64, max int64) (int64, error) {
	rawValue, ok := values[key]
	if !ok || isNullJSON(rawValue) {
		return 0, errInvalidInput
	}

	var parsed int64
	if err := json.Unmarshal(rawValue, &parsed); err != nil || parsed < min || parsed > max {
		return 0, errInvalidInput
	}

	return parsed, nil
}

func parseRequiredNumber(values rawPayload, key string, min float64, max float64) (float64, error) {
	rawValue, ok := values[key]
	if !ok || isNullJSON(rawValue) {
		return 0, errInvalidInput
	}

	var parsed float64
	if err := json.Unmarshal(rawValue, &parsed); err != nil || parsed < min || parsed > max {
		return 0, errInvalidInput
	}

	return parsed, nil
}

func isNullJSON(rawValue json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(rawValue), jsonNull)
}

func newRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

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
	gin.SetMode(gin.ReleaseMode)
	router := newRouter()
	router.Run(":8000")
}
