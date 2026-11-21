package xsd

import "testing"

// Offsets outside Go 1.23+'s accepted range must be rejected on every Go
// version, so validation results do not depend on the toolchain.
func TestDateTimeOffsetRange(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"2026-11-21T10:30:00+03:00", true},
		{"2026-11-21T10:30:00-03:00", true},
		{"2026-11-21T10:30:00+24:00", true},
		{"2026-11-21T10:30:00.123Z", true},
		{"2026-11-21T10:30:00", true},
		{"2026-11-21T10:30:00+25:00", false},
		{"2026-11-21T10:30:00+23:61", false},
		{"2026-11-21T10:30:00-99:00", false},
	}
	for _, c := range cases {
		_, err := DateTime(c.in).Time()
		if (err == nil) != c.ok {
			t.Errorf("DateTime(%q).Time() error = %v, want ok=%v", c.in, err, c.ok)
		}
	}
}
