package api

import (
	"cristhianflo/language-royale/internal/score"
	"strings"
)

type rawSignals struct {
	LprHits24h        *int64   `json:"lpr_hits_24h"`
	AddressMatch      *bool    `json:"address_match"`
	DistanceMiles     *float64 `json:"distance_miles"`
	DaysSinceLastSeen *int64   `json:"days_since_last_seen"`
}

type rawInput struct {
	CaseID  *string     `json:"case_id"`
	Signals *rawSignals `json:"signals"`
}

func validateInput(raw rawInput) (score.Input, error) {
	if raw.CaseID == nil || strings.TrimSpace(*raw.CaseID) == "" {
		return score.Input{}, score.ErrInvalidInput
	}

	if raw.Signals == nil {
		return score.Input{}, score.ErrInvalidInput
	}

	sig := raw.Signals
	if sig.LprHits24h == nil || *sig.LprHits24h < 0 || *sig.LprHits24h > 10000 {
		return score.Input{}, score.ErrInvalidInput
	}

	if sig.AddressMatch == nil {
		return score.Input{}, score.ErrInvalidInput
	}

	if sig.DistanceMiles == nil || *sig.DistanceMiles < 0 || *sig.DistanceMiles > 10000 {
		return score.Input{}, score.ErrInvalidInput
	}

	if sig.DaysSinceLastSeen == nil || *sig.DaysSinceLastSeen < 0 || *sig.DaysSinceLastSeen > 36500 {
		return score.Input{}, score.ErrInvalidInput
	}

	return score.Input{
		CaseID:            *raw.CaseID,
		LprHits24h:        *sig.LprHits24h,
		AddressMatch:      *sig.AddressMatch,
		DistanceMiles:     *sig.DistanceMiles,
		DaysSinceLastSeen: *sig.DaysSinceLastSeen,
	}, nil
}
