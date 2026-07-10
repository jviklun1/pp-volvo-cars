package domain

// Market is a canonical Volvo market identifier, such as "se" or "gb".
type Market string

// ModelSlug is a stable market model identifier.
type ModelSlug string

// ModelYear is a calendar model year.
type ModelYear int

// Powertrain is a normalized powertrain identifier.
type Powertrain string

// Language is a BCP-47 language tag.
type Language string

// Applicability describes one vehicle slice to which a knowledge item applies.
// A nil field is a wildcard. An item can carry zero or many applicability rows;
// zero rows means that it applies to every vehicle in its market.
type Applicability struct {
	ModelSlug  *ModelSlug
	ModelYear  *ModelYear
	Powertrain *Powertrain
}

// ApplicabilityFilter is the optional vehicle context supplied by a caller.
type ApplicabilityFilter struct {
	ModelSlug  *ModelSlug
	ModelYear  *ModelYear
	Powertrain *Powertrain
}

// Matches reports whether a single applicability row admits the filter.
func (a Applicability) Matches(filter ApplicabilityFilter) bool {
	return matches(a.ModelSlug, filter.ModelSlug) &&
		matches(a.ModelYear, filter.ModelYear) &&
		matches(a.Powertrain, filter.Powertrain)
}

// MatchesApplicability reports whether any row admits the filter. No rows is
// the canonical representation for an item that applies to all vehicles.
func MatchesApplicability(rows []Applicability, filter ApplicabilityFilter) bool {
	if len(rows) == 0 {
		return true
	}
	for _, row := range rows {
		if row.Matches(filter) {
			return true
		}
	}
	return false
}

func matches[T comparable](constraint, requested *T) bool {
	return constraint == nil || requested == nil || *constraint == *requested
}
