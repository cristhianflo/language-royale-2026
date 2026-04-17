package score

import "math"

func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

func sigmoid(value float64) float64 {
	return 1 / (1 + math.Exp(-value))
}

func CalculateScore(input Input) float64 {
	addressMatch := 0.0
	if input.AddressMatch {
		addressMatch = 1.2
	}

	rawScore := 0.9*float64(input.LprHits24h) + addressMatch - 0.35*input.DistanceMiles - 0.6*float64(input.DaysSinceLastSeen)
	return roundToTwoDecimals(sigmoid(rawScore))
}

func GetTier(score float64) string {
	if score >= 0.80 {
		return "HOT"
	}
	if score >= 0.50 {
		return "WARM"
	}
	return "COLD"
}
