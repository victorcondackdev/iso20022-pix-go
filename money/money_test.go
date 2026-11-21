package money

import (
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Amount
	}{
		{"0", 0},
		{"0.0", 0},
		{"0.00", 0},
		{"0.01", 1},
		{"0.1", 10},
		{"1", 100},
		{"10.5", 1050},
		{"10.50", 1050},
		{"010.50", 1050},
		{"1234567.89", 123456789},
		{"9999999999999999.99", 999999999999999999},
		{"0000000000000000.01", 1},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{
		"", ".", ".5", "1.", "1.234", "-1", "+1", "1e2", "1E2", " 1", "1 ",
		"1,00", "1.0.0", "1_000", "0x10", "NaN", "Inf", "１",
		"1234567890123456789",   // 19 digits
		"12345678901234567.890", // 20 digits
		"99999999999999999.9",   // 18 digits but above MaxAmount
		"999999999999999999",    // 18 integer digits
		"10000000000000000",     // MaxAmount + 0.01
	} {
		_, err := Parse(in)
		if err == nil {
			t.Errorf("Parse(%q): expected error", in)
			continue
		}
		if !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("Parse(%q): error %v does not wrap ErrInvalidAmount", in, err)
		}
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		in   Amount
		want string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{10, "0.10"},
		{100, "1.00"},
		{1050, "10.50"},
		{-1, "-0.01"},
		{-1050, "-10.50"},
		{math.MaxInt64, "92233720368547758.07"},
		{math.MinInt64, "-92233720368547758.08"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("Amount(%d).String() = %q, want %q", int64(tt.in), got, tt.want)
		}
	}
}

func TestText(t *testing.T) {
	type el struct {
		XMLName xml.Name `xml:"Amt"`
		Ccy     string   `xml:"Ccy,attr"`
		Value   Amount   `xml:",chardata"`
	}
	b, err := xml.Marshal(el{Ccy: "BRL", Value: 1050})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `<Amt Ccy="BRL">10.50</Amt>` {
		t.Fatalf("got %s", b)
	}
	var back el
	if err := xml.Unmarshal([]byte(`<Amt Ccy="BRL">7.5</Amt>`), &back); err != nil {
		t.Fatal(err)
	}
	if back.Value != 750 {
		t.Fatalf("got %d", back.Value)
	}
	if err := xml.Unmarshal([]byte(`<Amt Ccy="BRL">7,5</Amt>`), &back); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("want ErrInvalidAmount, got %v", err)
	}
	if _, err := xml.Marshal(el{Ccy: "BRL", Value: -1}); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("negative marshal: want ErrInvalidAmount, got %v", err)
	}
	if _, err := xml.Marshal(el{Ccy: "BRL", Value: MaxAmount + 1}); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("huge marshal: want ErrInvalidAmount, got %v", err)
	}
	if b, err := MaxAmount.MarshalText(); err != nil || string(b) != "9999999999999999.99" {
		t.Fatalf("MaxAmount: %s %v", b, err)
	}
}

func TestArithmetic(t *testing.T) {
	a, b := MustParse("0.1"), MustParse("0.2")
	s, err := a.Add(b)
	if err != nil || s != MustParse("0.30") {
		t.Fatalf("0.1+0.2 = %v, %v", s, err)
	}
	d, err := s.Sub(MustParse("0.30"))
	if err != nil || !d.IsZero() || d.IsPositive() {
		t.Fatalf("0.30-0.30 = %v, %v", d, err)
	}
	if _, err := Amount(math.MaxInt64).Add(1); !errors.Is(err, ErrOverflow) {
		t.Errorf("MaxInt64+1: %v", err)
	}
	if _, err := Amount(math.MinInt64).Add(-1); !errors.Is(err, ErrOverflow) {
		t.Errorf("MinInt64-1 via Add: %v", err)
	}
	if _, err := Amount(math.MinInt64).Sub(1); !errors.Is(err, ErrOverflow) {
		t.Errorf("MinInt64-1: %v", err)
	}
	if _, err := Amount(math.MaxInt64).Sub(-1); !errors.Is(err, ErrOverflow) {
		t.Errorf("MaxInt64+1 via Sub: %v", err)
	}
	if FromCents(42).Cents() != 42 {
		t.Error("FromCents/Cents")
	}
}

func TestMustParsePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	MustParse("1.234")
}

// reference is an independent implementation of the accepted grammar used
// to cross-check Parse in the fuzz test.
func reference(s string) (int64, bool) {
	intPart, frac, hasDot := strings.Cut(s, ".")
	if intPart == "" || (hasDot && (frac == "" || len(frac) > 2)) {
		return 0, false
	}
	for _, r := range intPart + frac {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	if len(intPart)+len(frac) > MaxTotalDigits {
		return 0, false
	}
	for len(frac) < 2 {
		frac += "0"
	}
	v, err := strconv.ParseInt(intPart+frac, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"0", "0.01", "10.5", "10.50", "1.234", "-1", "1e3", "92233720368547758.07", "00000000000000001.1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		a, err := Parse(s)
		want, ok := reference(s)
		if ok && want > int64(MaxAmount) {
			ok = false
		}
		if (err == nil) != ok {
			t.Fatalf("Parse(%q) err=%v, reference ok=%v", s, err, ok)
		}
		if err != nil {
			return
		}
		if int64(a) != want {
			t.Fatalf("Parse(%q) = %d, reference %d", s, a, want)
		}
		back, err := Parse(a.String())
		if err != nil || back != a {
			t.Fatalf("round trip %q -> %q -> %v, %v", s, a.String(), back, err)
		}
	})
}

func FuzzFormat(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1))
	f.Add(int64(math.MaxInt64))
	f.Fuzz(func(t *testing.T, c int64) {
		a := FromCents(c)
		s := a.String()
		if c < 0 || a > MaxAmount {
			if _, err := a.MarshalText(); err == nil {
				t.Fatalf("MarshalText(%d) accepted an out-of-range amount", c)
			}
			return
		}
		back, err := Parse(s)
		if err != nil || back != a {
			t.Fatalf("%d -> %q -> %v, %v", c, s, back, err)
		}
		if want := fmt.Sprintf("%d.%02d", c/100, c%100); s != want {
			t.Fatalf("String(%d) = %q, want %q", c, s, want)
		}
	})
}
