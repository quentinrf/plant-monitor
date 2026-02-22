package domain

import (
	"strings"
	"testing"
)

func TestAnalyze_LowLight(t *testing.T) {
	result := Analyze(100.0, nil)

	if result.Category != "Low Light" {
		t.Errorf("category: got %q, want %q", result.Category, "Low Light")
	}
	if !strings.Contains(result.Recommendation, "houseplants") {
		t.Errorf("recommendation %q should mention houseplants", result.Recommendation)
	}
	if result.CurrentLux != 100.0 {
		t.Errorf("CurrentLux: got %v, want 100.0", result.CurrentLux)
	}
}

func TestAnalyze_MediumLight(t *testing.T) {
	result := Analyze(1000.0, nil)

	if result.Category != "Medium Light" {
		t.Errorf("category: got %q, want %q", result.Category, "Medium Light")
	}
	if !strings.Contains(result.Recommendation, "tropical") {
		t.Errorf("recommendation %q should mention tropical", result.Recommendation)
	}
}

func TestAnalyze_HighLight(t *testing.T) {
	result := Analyze(5000.0, nil)

	if result.Category != "High Light" {
		t.Errorf("category: got %q, want %q", result.Category, "High Light")
	}
	if !strings.Contains(result.Recommendation, "succulents") {
		t.Errorf("recommendation %q should mention succulents", result.Recommendation)
	}
}

func TestAnalyze_Trend_Brightening(t *testing.T) {
	// Doubles from 100 → 200: second half avg clearly higher than first.
	history := []float64{100, 110, 150, 180, 200}
	result := Analyze(200.0, history)

	if result.Trend != "brightening" {
		t.Errorf("trend: got %q, want %q", result.Trend, "brightening")
	}
}

func TestAnalyze_Trend_Darkening(t *testing.T) {
	// Halves from 200 → 100: second half avg clearly lower than first.
	history := []float64{200, 180, 150, 110, 100}
	result := Analyze(100.0, history)

	if result.Trend != "darkening" {
		t.Errorf("trend: got %q, want %q", result.Trend, "darkening")
	}
}

func TestAnalyze_Trend_Stable(t *testing.T) {
	history := []float64{500, 505, 498, 502, 500}
	result := Analyze(500.0, history)

	if result.Trend != "stable" {
		t.Errorf("trend: got %q, want %q", result.Trend, "stable")
	}
}

func TestAnalyze_EmptyHistory(t *testing.T) {
	// Must not panic; AverageLux should fall back to CurrentLux.
	result := Analyze(300.0, nil)

	if result.Trend != "stable" {
		t.Errorf("trend with no history: got %q, want %q", result.Trend, "stable")
	}
	if result.AverageLux != 300.0 {
		t.Errorf("AverageLux with no history: got %v, want 300.0", result.AverageLux)
	}
	if result.Recommendation == "" {
		t.Error("Recommendation should not be empty")
	}
}
