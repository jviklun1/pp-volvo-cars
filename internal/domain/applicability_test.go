package domain

import "testing"

func TestApplicabilityNullMeansAll(t *testing.T) {
	model := ModelSlug("ex30")
	year := ModelYear(2026)
	powertrain := Powertrain("electric")
	filter := ApplicabilityFilter{ModelSlug: &model, ModelYear: &year, Powertrain: &powertrain}

	tests := []struct {
		name string
		rows []Applicability
		want bool
	}{
		{name: "no rows apply to all", rows: nil, want: true},
		{name: "all-null row applies to all", rows: []Applicability{{}}, want: true},
		{name: "matching constrained row", rows: []Applicability{{ModelSlug: &model}}, want: true},
		{name: "different constrained row", rows: []Applicability{{ModelSlug: ptr(ModelSlug("xc90"))}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesApplicability(tt.rows, filter); got != tt.want {
				t.Fatalf("MatchesApplicability() = %v, want %v", got, tt.want)
			}
		})
	}
}

func ptr[T any](value T) *T {
	return &value
}
