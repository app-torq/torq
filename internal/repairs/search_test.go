package repairs

import (
	"reflect"
	"testing"
)

func TestParseSearch(t *testing.T) {
	for _, tc := range []struct {
		q    string
		want Search
	}{
		{"", Search{}},
		{"weak braking", Search{Text: "weak braking"}},
		{"tags:brake-fluid,brake-disks insufficient braking power", Search{Tags: []string{"brake-fluid", "brake-disks"}, Text: "insufficient braking power"}},
		{"noise TAGS:abs", Search{Tags: []string{"abs"}, Text: "noise"}},
		{"tags: abs,esp rattle", Search{Tags: []string{"abs", "esp"}, Text: "rattle"}},
		{`tag:"brake fluid",abs  "front left"`, Search{Tags: []string{"brake fluid", "abs"}, Text: "front left"}},
		{"tags:a tags:b", Search{Tags: []string{"a", "b"}}},
		{"tags:,, P0300", Search{Text: "P0300"}},
		{"tags:", Search{}},
		{"oem:123 https://x", Search{Text: "oem:123 https://x"}},
	} {
		if got := ParseSearch(tc.q); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseSearch(%q) = %+v, want %+v", tc.q, got, tc.want)
		}
	}
}
