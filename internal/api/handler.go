package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"cristhianflo/language-royale/internal/score"
	"github.com/gin-gonic/gin"
)

func scoreHandler(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": score.ErrInvalidInput.Error()})
		return
	}

	bodyBytes = bytes.TrimSpace(bodyBytes)
	if len(bodyBytes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": score.ErrInvalidInput.Error()})
		return
	}

	// Batch processing
	if bodyBytes[0] == '[' {
		var rawBatch []rawInput
		if err := json.Unmarshal(bodyBytes, &rawBatch); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": score.ErrInvalidInput.Error()})
			return
		}

		results := make([]any, len(rawBatch))
		var wg sync.WaitGroup

		for i, raw := range rawBatch {
			wg.Add(1)
			go func(idx int, r rawInput) {
				defer wg.Done()
				input, err := validateInput(r)
				if err != nil {
					caseID := ""
					if r.CaseID != nil {
						caseID = *r.CaseID
					}
					results[idx] = gin.H{
						"case_id": caseID,
						"error":   err.Error(),
					}
					return
				}

				scoreVal := score.CalculateScore(input)
				results[idx] = score.CaseResponse{
					CaseID: input.CaseID,
					Score:  scoreVal,
					Tier:   score.GetTier(scoreVal),
				}
			}(i, raw)
		}

		wg.Wait()
		c.JSON(http.StatusOK, results)
		return
	}

	// Single processing
	if bodyBytes[0] == '{' {
		var raw rawInput
		decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
		if err := decoder.Decode(&raw); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": score.ErrInvalidInput.Error()})
			return
		}

		// Ensure no extra trailing JSON objects
		if decoder.More() {
			c.JSON(http.StatusBadRequest, gin.H{"error": score.ErrInvalidInput.Error()})
			return
		}

		input, err := validateInput(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": score.ErrInvalidInput.Error()})
			return
		}

		scoreVal := score.CalculateScore(input)
		resp := score.CaseResponse{
			CaseID: input.CaseID,
			Score:  scoreVal,
			Tier:   score.GetTier(scoreVal),
		}

		c.JSON(http.StatusOK, resp)
		return
	}

	// Neither a JSON object nor an array
	c.JSON(http.StatusBadRequest, gin.H{"error": score.ErrInvalidInput.Error()})
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok": true,
	})
}
