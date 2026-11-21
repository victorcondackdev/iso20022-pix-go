package check

import (
	"strings"
	"testing"

	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

func TestChecker(t *testing.T) {
	var c Checker
	if c.Err() != nil {
		t.Fatal("empty checker returned an error")
	}
	c.Date("/d", "", true)
	c.Date("/d2", "", false)
	c.DateTime("/dt", "", false)
	c.DateOrDateTime("/dd", nil, true)
	c.OneOf("/o", "", false, "A")
	c.Text("/t", "", 3, false)
	c.Account("/acc", nil, false)
	c.Account("/acc2", &xsd.Account{Id: xsd.AccountID{IBAN: "BR123"}, Tp: xsd.Proprietary("CACC")}, true)
	c.Choice("/ch", nil, 35)
	c.Remittance("/r", nil)
	c.Party("/p", xsd.Party{Nm: "x"}, true, false)
	c.Account("/acc3", &xsd.Account{Id: xsd.AccountID{Othr: &xsd.GenericAccountID{Id: "1"}}, Tp: &xsd.CodeOrProprietary{Cd: "CACC", Prtry: "CACC"}}, true)
	c.NbOfTxs("/n", "1234567890123456", 1)
	c.ReturnID("/rid", "")
	c.Text("/ctl", "a\x00b", 10, true)
	err := c.Err()
	if err == nil {
		t.Fatal("expected violations")
	}
	want := []string{"/d: is required", "/dd: is required", "/acc3/Tp: must contain exactly one of Cd or Prtry, got both", "/n: must be a number", "/rid: is required", "/ctl: contains U+0000, which XML 1.0 does not allow"}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("missing %q in %v", w, err)
		}
	}
	if n := len(err.(xsd.Violations)); n != len(want) {
		t.Errorf("got %d violations: %v", n, err)
	}
	if trimPrefix("ab", "abc") != "ab" {
		t.Error("trimPrefix")
	}
}
