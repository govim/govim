package govim

import "testing"

func TestParseVersionLong(t *testing.T) {
	testVals := []struct {
		in   int
		want string
	}{
		{in: 8020000, want: "v8.2.0"},
		{in: 8011711, want: "v8.1.1711"},
	}
	for _, v := range testVals {
		got := ParseVersionLong(v.in)
		if got != v.want {
			t.Errorf("ParseVersionLong(%v) gave %q; want %q", v.in, got, v.want)
		}
	}
}
