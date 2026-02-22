package domain

// LightAnalysis is the result of analyzing a set of light readings.
type LightAnalysis struct {
	CurrentLux     float64
	AverageLux     float64
	Trend          string // "stable" | "brightening" | "darkening"
	Category       string
	Recommendation string
}

// Light level thresholds in lux — kept in sync with light-service domain.
const (
	LowLightMax    = 200.0  // below this → Low Light
	MediumLightMax = 2500.0 // below this → Medium Light; at or above → High Light
)

// Analyze produces a LightAnalysis from a current lux value and a slice of
// historical lux readings (oldest first). An empty history is valid; in that
// case trend defaults to "stable" and AverageLux equals CurrentLux.
func Analyze(currentLux float64, history []float64) LightAnalysis {
	category := categoryFromLux(currentLux)

	avgLux := currentLux
	trend := "stable"

	if len(history) > 0 {
		avgLux = avgSlice(history)
		trend = computeTrend(history)
	}

	return LightAnalysis{
		CurrentLux:     currentLux,
		AverageLux:     avgLux,
		Trend:          trend,
		Category:       category,
		Recommendation: recommendationForCategory(category),
	}
}

// computeTrend compares the average of the first half of readings against the
// second half. A change of more than 10% is considered a trend.
func computeTrend(history []float64) string {
	if len(history) < 2 {
		return "stable"
	}

	mid := len(history) / 2
	firstAvg := avgSlice(history[:mid])
	secondAvg := avgSlice(history[mid:])

	if firstAvg == 0 {
		return "stable"
	}

	change := (secondAvg - firstAvg) / firstAvg
	switch {
	case change > 0.10:
		return "brightening"
	case change < -0.10:
		return "darkening"
	default:
		return "stable"
	}
}

// categoryFromLux maps a lux value to a human-readable light category.
func categoryFromLux(lux float64) string {
	switch {
	case lux < LowLightMax:
		return "Low Light"
	case lux < MediumLightMax:
		return "Medium Light"
	default:
		return "High Light"
	}
}

// recommendationForCategory maps a light category to plant care advice.
func recommendationForCategory(category string) string {
	switch category {
	case "Low Light":
		return "Low Light — most houseplants will thrive here"
	case "Medium Light":
		return "Medium Light — ideal for most tropical plants"
	default:
		return "High Light — perfect for succulents and cacti"
	}
}

// avgSlice returns the arithmetic mean of a non-empty slice. Callers must
// ensure the slice is non-empty before calling.
func avgSlice(vals []float64) float64 {
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
