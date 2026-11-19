package domain

import "testing"

func TestParsePriceRange(t *testing.T) {
	tests := []struct {
		raw     string
		want    PriceRange
		wantErr bool
	}{
		{raw: "1M", want: Range1M},
		{raw: "3M", want: Range3M},
		{raw: "1Y", want: Range1Y},
		{raw: "", wantErr: true},
		{raw: "1m", wantErr: true},
		{raw: "2M", wantErr: true},
		{raw: "1Y ", wantErr: true},
	}
	for _, test := range tests {
		got, err := ParsePriceRange(test.raw)
		if test.wantErr {
			if err == nil || !IsCode(err, CodeInvalidRange) {
				t.Fatalf("ParsePriceRange(%q) error = %v, want INVALID_RANGE", test.raw, err)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Fatalf("ParsePriceRange(%q) = %q, %v", test.raw, got, err)
		}
	}
}

func TestPointsFor(t *testing.T) {
	tests := map[PriceRange]int{Range1M: 30, Range3M: 90, Range1Y: 365, PriceRange("x"): 0}
	for priceRange, want := range tests {
		if got := PointsFor(priceRange); got != want {
			t.Fatalf("PointsFor(%q) = %d, want %d", priceRange, got, want)
		}
	}
}

func TestOrderOldestFirst(t *testing.T) {
	input := []PricePoint{
		{Date: "2026-11-14", Close: 3},
		{Date: "2026-11-12", Close: 1},
		{Date: "2026-11-13", Close: 2},
	}
	ordered := OrderOldestFirst(input)
	want := []string{"2026-11-12", "2026-11-13", "2026-11-14"}
	for index, date := range want {
		if ordered[index].Date != date {
			t.Fatalf("index %d = %q, want %q", index, ordered[index].Date, date)
		}
	}
	if input[0].Date != "2026-11-14" {
		t.Fatal("input slice was reordered")
	}
}
