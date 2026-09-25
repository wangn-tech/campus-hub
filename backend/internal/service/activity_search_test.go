package service

import "testing"

func TestValidateActivitySearch(t *testing.T) {
	lon, lat := 116.4, 39.9
	status := 2
	cases := []struct {
		name    string
		query   ActivitySearchQuery
		wantErr bool
	}{
		{"valid geo", ActivitySearchQuery{Status: &status, Longitude: &lon, Latitude: &lat, Distance: "5km", Sort: "distance"}, false},
		{"private status", ActivitySearchQuery{Status: intPtr(1)}, true},
		{"bad distance", ActivitySearchQuery{Longitude: &lon, Latitude: &lat, Distance: "near"}, true},
		{"one coordinate", ActivitySearchQuery{Longitude: &lon}, true},
		{"distance sort no point", ActivitySearchQuery{Sort: "distance"}, true},
		{"bad sort", ActivitySearchQuery{Sort: "random"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if (validateActivitySearch(tc.query) != nil) != tc.wantErr {
				t.Fatalf("validateActivitySearch(%+v)", tc.query)
			}
		})
	}
}
func intPtr(value int) *int { return &value }
