package score

import "errors"

type CaseResponse struct {
	CaseID string  `json:"case_id"`
	Score  float64 `json:"score"`
	Tier   string  `json:"tier"`
}

type Input struct {
	CaseID            string
	LprHits24h        int64
	AddressMatch      bool
	DistanceMiles     float64
	DaysSinceLastSeen int64
}

var ErrInvalidInput = errors.New("invalid input")
