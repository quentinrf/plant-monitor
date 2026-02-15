package domain

import (
	"testing"
)

func TestNewLightReading(t *testing.T) {
	tests := []struct {
		name    string
		lux     float64
		wantErr bool
	}{
		{
			name:    "valid reading",
			lux:     500.0,
			wantErr: false,
		},
		{
			name:    "zero lux is valid",
			lux:     0.0,
			wantErr: false,
		},
		{
			name:    "negative lux is invalid",
			lux:     -10.0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reading, err := NewLightReading(tt.lux)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if reading.Lux != tt.lux {
				t.Errorf("expected lux %v, got %v", tt.lux, reading.Lux)
			}
		})
	}
}

func TestLightReading_IsLowLight(t *testing.T) {
	tests := []struct {
		lux  float64
		want bool
	}{
		{lux: 100, want: true},
		{lux: 199, want: true},
		{lux: 200, want: false},
		{lux: 500, want: false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			reading, _ := NewLightReading(tt.lux)
			if got := reading.IsLowLight(); got != tt.want {
				t.Errorf("IsLowLight() = %v, want %v for lux %v", got, tt.want, tt.lux)
			}
		})
	}
}

func TestLightReading_LightCategory(t *testing.T) {
	tests := []struct {
		lux  float64
		want string
	}{
		{lux: 100, want: "Low Light"},
		{lux: 500, want: "Medium Light"},
		{lux: 3000, want: "High Light"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			reading, _ := NewLightReading(tt.lux)
			if got := reading.LightCategory(); got != tt.want {
				t.Errorf("LightCategory() = %v, want %v for lux %v", got, tt.want, tt.lux)
			}
		})
	}
}
