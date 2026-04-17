package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"cristhianflo/language-royale/internal/score"
	"github.com/gin-gonic/gin"
)

const scoreFixtureSampleCount = 10

type scoreFixture struct {
	Input      json.RawMessage     `json:"input"`
	Output     *score.CaseResponse `json:"output"`
	StatusCode int                 `json:"status_code"`
	lineNumber int
}

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if !body["ok"] {
		t.Fatalf("expected ok=true, got %#v", body)
	}
}

func TestScoreFixturesFirstTen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter()
	fixtures := loadScoreFixtures(t, scoreFixtureSampleCount)

	for _, fixture := range fixtures {
		t.Run(fixtureName(fixture), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/score", bytes.NewReader(fixture.Input))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != fixture.StatusCode {
				t.Fatalf("expected status %d, got %d with body %s", fixture.StatusCode, rec.Code, rec.Body.String())
			}

			if fixture.StatusCode != http.StatusOK {
				return
			}

			if fixture.Output == nil {
				t.Fatalf("fixture line %d is missing expected output", fixture.lineNumber)
			}

			var got score.CaseResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("failed to decode score response: %v", err)
			}

			if got.CaseID != fixture.Output.CaseID {
				t.Fatalf("expected case_id %q, got %q", fixture.Output.CaseID, got.CaseID)
			}

			if got.Tier != fixture.Output.Tier {
				t.Fatalf("expected tier %q, got %q", fixture.Output.Tier, got.Tier)
			}

			if math.Abs(got.Score-fixture.Output.Score) > 1e-9 {
				t.Fatalf("expected score %.2f, got %.2f", fixture.Output.Score, got.Score)
			}
		})
	}
}

func TestScoreHackFixturesStatusOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter()

	file, err := os.Open(filepath.Join("..", "..", "hack", "problem_a_generate.jsonl"))
	if err != nil {
		t.Fatalf("failed to open hack fixture file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		var fixture scoreFixture
		if err := json.Unmarshal(scanner.Bytes(), &fixture); err != nil {
			t.Fatalf("failed to decode hack fixture line %d: %v", lineNumber, err)
		}

		req := httptest.NewRequest(http.MethodPost, "/score", bytes.NewReader(fixture.Input))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != fixture.StatusCode {
			t.Fatalf("hack fixture line %d expected status %d, got %d with body %s", lineNumber, fixture.StatusCode, rec.Code, rec.Body.String())
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan hack fixture file: %v", err)
	}
}

func loadScoreFixtures(t *testing.T, limit int) []scoreFixture {
	t.Helper()

	file, err := os.Open(scoreFixturePath())
	if err != nil {
		t.Fatalf("failed to open fixture file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	fixtures := make([]scoreFixture, 0, limit)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		var fixture scoreFixture
		if err := json.Unmarshal(scanner.Bytes(), &fixture); err != nil {
			t.Fatalf("failed to decode fixture line %d: %v", lineNumber, err)
		}

		fixture.lineNumber = lineNumber
		fixtures = append(fixtures, fixture)

		if len(fixtures) == limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan fixture file: %v", err)
	}

	if len(fixtures) != limit {
		t.Fatalf("expected %d fixtures, loaded %d", limit, len(fixtures))
	}

	return fixtures
}

func scoreFixturePath() string {
	return filepath.Join("..", "..", "hack", "api_testcases_subset.jsonl")
}

func fixtureName(fixture scoreFixture) string {
	return "line_" + strconv.Itoa(fixture.lineNumber) + "_status_" + strconv.Itoa(fixture.StatusCode)
}
