package model

import (
	"strings"
	"testing"
)

func TestValidation(t *testing.T) {
	for _, tc := range []struct {
		name, address, segment string
		bad                    bool
	}{
		{"unicode limit", strings.Repeat("Я", 255), strings.Repeat("Ю", 16), false},
		{"long address", strings.Repeat("Я", 256), "A", true},
		{"long segment", "x", strings.Repeat("Ю", 17), true},
		{"empty address", "", "A", true}, {"blank address", " \t", "A", true},
		{"NUL address", "x\x00", "A", true}, {"NUL segment", "x", "A\x00", true},
		{"empty segment", "x", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (Segmentation{AddressSAPID: tc.address, AdrSegment: tc.segment}).Validate()
			if (err != nil) != tc.bad {
				t.Fatalf("unexpected validation: %v", err)
			}
		})
	}
}
