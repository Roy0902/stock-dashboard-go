package domain

import "sort"

// PriceRange is one of the three supported chart windows.
type PriceRange string

const (
	Range1M PriceRange = "1M"
	Range3M PriceRange = "3M"
	Range1Y PriceRange = "1Y"
)

// PricePoint is one daily close. Date is an ISO-8601 calendar date (YYYY-MM-DD).
type PricePoint struct {
	Date  string
	Close float64
}

// ParsePriceRange accepts exactly "1M", "3M" or "1Y". The comparison is
// case-sensitive and an empty string is INVALID_RANGE.
func ParsePriceRange(raw string) (PriceRange, error) {
	switch PriceRange(raw) {
	case Range1M:
		return Range1M, nil
	case Range3M:
		return Range3M, nil
	case Range1Y:
		return Range1Y, nil
	default:
		return "", NewError(CodeInvalidRange, DefaultMessage(CodeInvalidRange))
	}
}

// PointsFor returns the requested number of trading days for a range.
func PointsFor(rangeValue PriceRange) int {
	switch rangeValue {
	case Range1M:
		return 30
	case Range3M:
		return 90
	case Range1Y:
		return 365
	default:
		return 0
	}
}

// OrderOldestFirst returns a new slice sorted ascending by ISO date. Provider payloads
// arrive newest first; the chart consumes oldest first.
func OrderOldestFirst(points []PricePoint) []PricePoint {
	out := make([]PricePoint, len(points))
	copy(out, points)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}
