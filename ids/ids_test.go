package ids

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

const sampleE2E = "E9999999920261121103000000000001"

func TestParseEndToEndID(t *testing.T) {
	id, err := ParseEndToEndID(sampleE2E)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 21, 10, 30, 0, 0, time.UTC)
	if id.Prefix != 'E' || id.ISPB != "99999999" || !id.Time.Equal(want) || id.Sequence != "00000000001" {
		t.Fatalf("unexpected %+v", id)
	}
	if id.String() != sampleE2E {
		t.Fatalf("String() = %q", id.String())
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "must be 32 characters"},
		{"D9999999920261121103000000000001", `must start with 'E'`},
		{"E99999A9920261121103000000000001", "ISPB"},
		{"E999999992026112110X000000000001", "timestamp"},
		{"E9999999920261321103000000000001", "not a valid date"},
		{"E9999999920260230103000000000001", "not a valid date"},
		{"E99999999202611211030000000000-1", "alphanumeric"},
	}
	for _, tt := range tests {
		_, err := ParseEndToEndID(tt.in)
		if err == nil || !errors.Is(err, ErrInvalidEndToEndID) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ParseEndToEndID(%q) = %v, want error containing %q", tt.in, err, tt.want)
		}
	}
	if _, err := ParseReturnID(sampleE2E); !errors.Is(err, ErrInvalidReturnID) {
		t.Errorf("ParseReturnID(E...) = %v", err)
	}
}

func TestNewIDs(t *testing.T) {
	at := time.Date(2026, 11, 21, 7, 30, 59, 0, time.FixedZone("BRT", -3*3600))
	e2e, err := NewEndToEndID("99999999", at, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseEndToEndID(e2e)
	if err != nil {
		t.Fatalf("%s: %v", e2e, err)
	}
	if p.Time.Format("200601021504") != "202611211030" {
		t.Errorf("time not converted to UTC: %s", e2e)
	}
	rtr, err := NewReturnID("12345678", at, bytes.NewReader(bytes.Repeat([]byte{0, 61, 255}, 20)))
	if err != nil {
		t.Fatal(err)
	}
	if rtr != "D123456782026112110300z0z0z0z0z0" {
		t.Errorf("deterministic RtrId = %q", rtr)
	}
	if _, err := ParseReturnID(rtr); err != nil {
		t.Error(err)
	}

	if _, err := NewEndToEndID("1234", at, nil); !errors.Is(err, ErrInvalidEndToEndID) {
		t.Errorf("short ISPB: %v", err)
	}
	if _, err := NewReturnID("12345678", time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), nil); !errors.Is(err, ErrInvalidReturnID) {
		t.Errorf("year 10000: %v", err)
	}
	if _, err := NewEndToEndID("12345678", at, bytes.NewReader(bytes.Repeat([]byte{255}, 4096))); err == nil {
		t.Error("expected error for unusable random source")
	}
	if _, err := NewEndToEndID("12345678", at, iotest.ErrReader(errors.New("boom"))); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("reader error: %v", err)
	}
}

func TestMsgID(t *testing.T) {
	m, err := NewMsgID("99999999", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 32 || m[0] != 'M' || m[1:9] != "99999999" || ValidateMsgID(m) != nil {
		t.Fatalf("bad MsgId %q", m)
	}
	if _, err := NewMsgID("x", nil); !errors.Is(err, ErrInvalidMsgID) {
		t.Errorf("bad ISPB: %v", err)
	}
	if _, err := NewMsgID("99999999", iotest.ErrReader(errors.New("boom"))); err == nil {
		t.Error("reader error not propagated")
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", 36), "\xff"} {
		if err := ValidateMsgID(bad); !errors.Is(err, ErrInvalidMsgID) {
			t.Errorf("ValidateMsgID(%q) = %v", bad, err)
		}
	}
	if err := ValidateMsgID(strings.Repeat("ã", 35)); err != nil {
		t.Errorf("35 runes rejected: %v", err)
	}
}

func TestTaxIDs(t *testing.T) {
	valid := map[string]func(string) bool{
		"52998224725":    ValidCPF,
		"11144477735":    ValidCPF,
		"11222333000181": ValidCNPJ,
		"12ABC34501DE35": ValidCNPJ,
	}
	for s, f := range valid {
		if !f(s) {
			t.Errorf("%s should be valid", s)
		}
	}
	for _, s := range []string{"52998224724", "11111111111", "5299822472", "5299822472a", "529.982.247-25"} {
		if ValidCPF(s) {
			t.Errorf("CPF %s should be invalid", s)
		}
	}
	for _, s := range []string{"11222333000182", "00000000000000", "12abc34501de35", "12ABC34501DEA5", "1122233300018", "11.222.333/0001-81"} {
		if ValidCNPJ(s) {
			t.Errorf("CNPJ %s should be invalid", s)
		}
	}
	if !ValidISPB("00000000") || ValidISPB("0000000") || ValidISPB("0000000a") {
		t.Error("ValidISPB")
	}
}

func TestClassifyPixKey(t *testing.T) {
	tests := []struct {
		in   string
		want KeyType
	}{
		{"52998224725", KeyCPF},
		{"11222333000181", KeyCNPJ},
		{"12ABC34501DE35", KeyCNPJ},
		{"fulano@example.com", KeyEmail},
		{"+5521987654321", KeyPhone},
		{"123e4567-e89b-12d3-a456-426614174000", KeyEVP},
	}
	for _, tt := range tests {
		got, err := ClassifyPixKey(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("ClassifyPixKey(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
		if got.String() == "UNKNOWN" {
			t.Errorf("%v has no name", got)
		}
	}
	for _, bad := range []string{
		"", "Fulano@example.com", "fulano@", "@example.com", "a@b@c.com", "a@nodot", "a@.com", "a@b.",
		"a b@example.com", strings.Repeat("a", 70) + "@example.com",
		"+0123", "+", "+55 21 98765-4321", "+1234567890123456",
		"123E4567-E89B-12D3-A456-426614174000", "123e4567-e89b-12d3-a456_426614174000",
		"52998224724", "529.982.247-25", "21987654321x",
	} {
		if k, err := ClassifyPixKey(bad); err == nil || !errors.Is(err, ErrInvalidPixKey) {
			t.Errorf("ClassifyPixKey(%q) = %v, %v; want error", bad, k, err)
		}
	}
	if KeyUnknown.String() != "UNKNOWN" {
		t.Error("KeyUnknown.String")
	}
}

func FuzzParseID(f *testing.F) {
	f.Add(sampleE2E)
	f.Add("D9999999920261121103000000000001")
	f.Add("E9999999920261321103000000000001")
	f.Fuzz(func(t *testing.T, s string) {
		for _, parse := range []func(string) (PixID, error){ParseEndToEndID, ParseReturnID} {
			id, err := parse(s)
			if err != nil {
				continue
			}
			if id.String() != s {
				t.Fatalf("round trip %q -> %q", s, id.String())
			}
			if id.Time.Location() != time.UTC || id.Time.Second() != 0 {
				t.Fatalf("time not UTC minute: %v", id.Time)
			}
		}
	})
}

func FuzzNewID(f *testing.F) {
	f.Add("99999999", int64(1795000000), []byte("seed"))
	f.Fuzz(func(t *testing.T, ispb string, unix int64, seed []byte) {
		at := time.Unix(unix%253402300799, 0) // keep within year 9999
		rnd := bytes.NewReader(bytes.Repeat(append(seed, 1), 64))
		s, err := NewEndToEndID(ispb, at, rnd)
		if err != nil {
			return
		}
		if _, err := ParseEndToEndID(s); err != nil {
			t.Fatalf("NewEndToEndID produced unparsable %q: %v", s, err)
		}
	})
}

func FuzzClassifyPixKey(f *testing.F) {
	f.Add("fulano@example.com")
	f.Add("+5521987654321")
	f.Fuzz(func(t *testing.T, s string) {
		k, err := ClassifyPixKey(s)
		if (err == nil) == (k == KeyUnknown) {
			t.Fatalf("ClassifyPixKey(%q) = %v, %v", s, k, err)
		}
	})
}
