package module

import (
	"testing"
)

func TestGetMostRestrictiveHeader(t *testing.T) {
	tests := []struct {
		name                     string
		prevCacheControlValue    string
		currentCacheControlValue string
		expected                 string
	}{
		{
			name:                     "Empty previous header",
			prevCacheControlValue:    "",
			currentCacheControlValue: "",
			expected:                 "max-age=0, private",
		},
		{
			name:                     "No-store in current header",
			prevCacheControlValue:    "max-age=3600, public",
			currentCacheControlValue: "no-store",
			expected:                 "max-age=3600, public",
		},
		{
			name:                     "No-cache in current header",
			prevCacheControlValue:    "max-age=3600, public",
			currentCacheControlValue: "no-cache",
			expected:                 "max-age=3600, public",
		},
		{
			name:                     "Current header has shorter max-age",
			prevCacheControlValue:    "max-age=3600, public",
			currentCacheControlValue: "max-age=1800, public",
			expected:                 "max-age=1800, public",
		},
		{
			name:                     "Previous header has shorter max-age",
			prevCacheControlValue:    "max-age=1800, public",
			currentCacheControlValue: "max-age=3600, public",
			expected:                 "max-age=1800, public",
		},
		{
			name:                     "Private visibility in previous header",
			prevCacheControlValue:    "max-age=3600, private",
			currentCacheControlValue: "max-age=3600, public",
			expected:                 "max-age=3600, private",
		},
		{
			name:                     "Private visibility in current header",
			prevCacheControlValue:    "max-age=3600, public",
			currentCacheControlValue: "max-age=3600, private",
			expected:                 "max-age=3600, private",
		},
		{
			name:                     "Private visibility in current header with shorter max-age",
			prevCacheControlValue:    "max-age=1800, public",
			currentCacheControlValue: "max-age=3600, private",
			expected:                 "max-age=1800, private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMostRestrictiveHeader(tt.prevCacheControlValue, tt.currentCacheControlValue)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
