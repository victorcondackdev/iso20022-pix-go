package xsd

import (
	"errors"
	"testing"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/money"
)

func TestDateTime(t *testing.T) {
	at := time.Date(2026, 11, 21, 7, 30, 0, 5e8, time.FixedZone("BRT", -3*3600))
	d := NewDateTime(at)
	if d != "2026-11-21T10:30:00.500Z" {
		t.Fatalf("NewDateTime = %q", d)
	}
	for _, s := range []DateTime{d, "2026-11-21T10:30:00Z", "2026-11-21T07:30:00-03:00", "2026-11-21T10:30:00", "2026-11-21T10:30:00.123456"} {
		if _, err := s.Time(); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	for _, s := range []DateTime{"", "2026-11-21", "2026-13-01T00:00:00Z", "21/11/2026 10:30"} {
		if _, err := s.Time(); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
	if NewDate(at) != "2026-11-21" {
		t.Error("NewDate")
	}
	if _, err := Date("2026-02-30").Time(); err == nil {
		t.Error("invalid date accepted")
	}
}

func TestHelpers(t *testing.T) {
	if BRL(money.MustParse("1.5")).String() != "1.50 BRL" {
		t.Error("Amount.String")
	}
	var nilCP *CodeOrProprietary
	if nilCP.Value() != "" || Code("A").Value() != "A" || Proprietary("P").Value() != "P" {
		t.Error("CodeOrProprietary.Value")
	}
	if (Party{}).TaxID() != "" || (Party{Id: &PartyID{OrgId: &OtherIDs{}}}).TaxID() != "" {
		t.Error("empty TaxID")
	}
	if Organisation("x", "11222333000181").TaxID() != "11222333000181" || Person("y", "52998224725").TaxID() != "52998224725" {
		t.Error("TaxID")
	}
	var a *Account
	if a.Number() != "" || a.Branch() != "" || a.PixKey() != "" {
		t.Error("nil account")
	}
	if (&Account{}).Number() != "" || (&Account{}).Branch() != "" {
		t.Error("empty account")
	}
	acc := NewAccount("0001", "123", AccountSavings)
	if acc.Number() != "123" || acc.Branch() != "0001" || acc.Tp.Prtry != "SVGS" {
		t.Error("NewAccount")
	}
	if (Agent{}).ISPB() != "" || AgentISPB("12345678").ISPB() != "12345678" {
		t.Error("Agent.ISPB")
	}
}

func TestViolations(t *testing.T) {
	if (Violations{}).Error() != "no violations" {
		t.Error("empty")
	}
	one := Violations{{Path: "/a", Message: "bad"}}
	if one.Error() != "/a: bad" {
		t.Error(one.Error())
	}
	two := append(one, Violation{Path: "/b", Message: "worse"})
	if two.Error() != "2 violations:\n\t/a: bad\n\t/b: worse" {
		t.Errorf("%q", two.Error())
	}
	var v Violation
	if !errors.As(error(two), &v) || v.Path != "/a" {
		t.Error("errors.As")
	}
}

func TestCodeSet(t *testing.T) {
	s := NewCodeSet(CodeInfo{Code: "B"}, CodeInfo{Code: "A"})
	if got := s.Sorted(); got[0].Code != "A" || got[1].Code != "B" {
		t.Error("Sorted")
	}
	if _, ok := s.Lookup("A"); !ok {
		t.Error("Lookup")
	}
}
