package service

import (
	"reflect"
	"sort"
	"testing"
)

func TestComputeCategories(t *testing.T) {
	tests := []struct {
		name    string
		aIntent []string
		bIntent []string
		want    []string
	}{
		{
			name:    "both cofounder",
			aIntent: []string{"cofounder"},
			bIntent: []string{"cofounder"},
			want:    []string{"cofounder"},
		},
		{
			name:    "both teammate",
			aIntent: []string{"teammate"},
			bIntent: []string{"teammate"},
			want:    []string{"teammate"},
		},
		{
			name:    "A is client, B is cofounder",
			aIntent: []string{"client"},
			bIntent: []string{"cofounder"},
			want:    []string{"client"},
		},
		{
			name:    "A is client, B is teammate",
			aIntent: []string{"client"},
			bIntent: []string{"teammate"},
			want:    []string{"client"},
		},
		{
			name:    "A is cofounder, B is client",
			aIntent: []string{"cofounder"},
			bIntent: []string{"client"},
			want:    []string{"client"},
		},
		{
			name:    "A is teammate, B is client",
			aIntent: []string{"teammate"},
			bIntent: []string{"client"},
			want:    []string{"client"},
		},
		{
			name:    "both client only — no match",
			aIntent: []string{"client"},
			bIntent: []string{"client"},
			want:    nil,
		},
		{
			name:    "no overlap at all",
			aIntent: []string{"cofounder"},
			bIntent: []string{"teammate"},
			want:    nil,
		},
		{
			name:    "all three intents each — all categories, client not duplicated",
			aIntent: []string{"cofounder", "teammate", "client"},
			bIntent: []string{"cofounder", "teammate", "client"},
			want:    []string{"cofounder", "teammate", "client"},
		},
		{
			name:    "empty intents",
			aIntent: nil,
			bIntent: nil,
			want:    nil,
		},
		{
			name:    "A empty, B has everything",
			aIntent: nil,
			bIntent: []string{"cofounder", "teammate", "client"},
			want:    nil,
		},
		{
			name:    "cofounder + client bidirectional client not duplicated",
			aIntent: []string{"cofounder", "client"},
			bIntent: []string{"cofounder", "client"},
			want:    []string{"cofounder", "client"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeCategories(tc.aIntent, tc.bIntent)

			// Normalize both to sorted slices for comparison
			sortedGot := append([]string{}, got...)
			sortedWant := append([]string{}, tc.want...)
			sort.Strings(sortedGot)
			sort.Strings(sortedWant)

			if !reflect.DeepEqual(sortedGot, sortedWant) {
				t.Errorf("computeCategories(%v, %v) = %v, want %v",
					tc.aIntent, tc.bIntent, got, tc.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		val   string
		want  bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", nil, "a", false},
		{"exact match only", []string{"abc"}, "ab", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := contains(tc.slice, tc.val)
			if got != tc.want {
				t.Errorf("contains(%v, %q) = %v, want %v", tc.slice, tc.val, got, tc.want)
			}
		})
	}
}
