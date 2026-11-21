// Package check implements the validation rules shared by the message
// packages. A Checker accumulates every violation instead of stopping at
// the first one.
package check

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/victorcondackdev/iso20022-pix-go/ids"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// Checker accumulates violations.
type Checker struct {
	vs xsd.Violations
}

// Addf records a violation at path.
func (c *Checker) Addf(path, format string, args ...any) {
	c.vs = append(c.vs, xsd.Violation{Path: path, Message: fmt.Sprintf(format, args...)})
}

// Err returns nil when nothing was recorded, otherwise the xsd.Violations.
func (c *Checker) Err() error {
	if len(c.vs) == 0 {
		return nil
	}
	return c.vs
}

// Idx formats "path[i]".
func Idx(path string, i int) string { return path + "[" + strconv.Itoa(i) + "]" }

// Text checks an ISO 20022 MaxNText value: valid UTF-8 and at most max
// characters; when required, at least one character.
func (c *Checker) Text(path, s string, max int, required bool) {
	if s == "" {
		if required {
			c.Addf(path, "is required")
		}
		return
	}
	if !utf8.ValidString(s) {
		c.Addf(path, "is not valid UTF-8")
		return
	}
	for _, r := range s {
		if !isXMLChar(r) {
			c.Addf(path, "contains %U, which XML 1.0 does not allow", r)
			return
		}
	}
	if n := utf8.RuneCountInString(s); n > max {
		c.Addf(path, "must be at most %d characters, got %d", max, n)
	}
}

// isXMLChar reports whether r is in the XML 1.0 Char production.
func isXMLChar(r rune) bool {
	return r == 0x09 || r == 0x0A || r == 0x0D ||
		(r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0x10FFFF)
}

// Max35 is Text(path, s, 35, required).
func (c *Checker) Max35(path, s string, required bool) { c.Text(path, s, 35, required) }

// OneOf checks that s, when non-empty, is one of allowed; when required it
// must be present.
func (c *Checker) OneOf(path, s string, required bool, allowed ...string) {
	if s == "" {
		if required {
			c.Addf(path, "is required")
		}
		return
	}
	for _, a := range allowed {
		if s == a {
			return
		}
	}
	c.Addf(path, "must be one of %v, got %q", allowed, s)
}

// Code checks an ISO 20022 external code (Max4Text-like, 1 to 4
// characters).
func (c *Checker) Code(path, s string, required bool) { c.Text(path, s, 4, required) }

// DateTime checks an ISODateTime.
func (c *Checker) DateTime(path string, d xsd.DateTime, required bool) {
	if d == "" {
		if required {
			c.Addf(path, "is required")
		}
		return
	}
	if _, err := d.Time(); err != nil {
		c.Addf(path, "must be an ISO 8601 date and time, got %q", string(d))
	}
}

// Date checks an ISODate.
func (c *Checker) Date(path string, d xsd.Date, required bool) {
	if d == "" {
		if required {
			c.Addf(path, "is required")
		}
		return
	}
	if _, err := d.Time(); err != nil {
		c.Addf(path, "must be an ISO 8601 date (YYYY-MM-DD), got %q", string(d))
	}
}

// DateOrDateTime checks a DateAndDateTime2Choice.
func (c *Checker) DateOrDateTime(path string, d *xsd.DateAndDateTime, required bool) {
	if d == nil {
		if required {
			c.Addf(path, "is required")
		}
		return
	}
	switch {
	case d.Dt != "" && d.DtTm != "":
		c.Addf(path, "must contain exactly one of Dt or DtTm, got both")
	case d.Dt == "" && d.DtTm == "":
		c.Addf(path, "must contain exactly one of Dt or DtTm")
	case d.Dt != "":
		c.Date(path+"/Dt", d.Dt, true)
	default:
		c.DateTime(path+"/DtTm", d.DtTm, true)
	}
}

// BRL checks that a is in BRL and, when positive is set, greater than
// zero. The value itself is already well-formed by construction.
func (c *Checker) BRL(path string, a xsd.Amount, positive bool) {
	if a.Ccy != xsd.CurrencyBRL {
		c.Addf(path+"/@Ccy", "must be %q, got %q", xsd.CurrencyBRL, a.Ccy)
	}
	switch {
	case a.Value < 0:
		c.Addf(path, "must not be negative")
	case a.Value > 999_999_999_999_999_999:
		c.Addf(path, "exceeds 18 digits")
	case positive && a.Value == 0:
		c.Addf(path, "must be greater than zero")
	}
}

// Choice checks a Cd/Prtry choice: exactly one present, Cd at most 4
// characters, Prtry at most maxPrtry.
func (c *Checker) Choice(path string, cp *xsd.CodeOrProprietary, maxPrtry int) {
	if cp == nil {
		return
	}
	switch {
	case cp.Cd != "" && cp.Prtry != "":
		c.Addf(path, "must contain exactly one of Cd or Prtry, got both")
	case cp.Cd == "" && cp.Prtry == "":
		c.Addf(path, "must contain exactly one of Cd or Prtry")
	case cp.Cd != "":
		c.Code(path+"/Cd", cp.Cd, true)
	default:
		c.Text(path+"/Prtry", cp.Prtry, maxPrtry, true)
	}
}

// EndToEndID checks a Pix EndToEndId.
func (c *Checker) EndToEndID(path, s string) {
	if s == "" {
		c.Addf(path, "is required")
		return
	}
	if _, err := ids.ParseEndToEndID(s); err != nil {
		c.Addf(path, "%s", trimPrefix(err.Error(), ids.ErrInvalidEndToEndID.Error()+": "))
	}
}

// ReturnID checks a Pix RtrId.
func (c *Checker) ReturnID(path, s string) {
	if s == "" {
		c.Addf(path, "is required")
		return
	}
	if _, err := ids.ParseReturnID(s); err != nil {
		c.Addf(path, "%s", trimPrefix(err.Error(), ids.ErrInvalidReturnID.Error()+": "))
	}
}

// Agent checks a DbtrAgt/CdtrAgt identified by ISPB.
func (c *Checker) Agent(path string, a xsd.Agent) {
	p := path + "/FinInstnId/ClrSysMmbId/MmbId"
	switch ispb := a.ISPB(); {
	case ispb == "":
		c.Addf(p, "is required (ISPB)")
	case !ids.ValidISPB(ispb):
		c.Addf(p, "must be an 8-digit ISPB, got %q", ispb)
	}
	if a.FinInstnId.ClrSysMmbId != nil {
		c.Choice(path+"/FinInstnId/ClrSysMmbId/ClrSysId", a.FinInstnId.ClrSysMmbId.ClrSysId, 35)
	}
}

// Party checks a Dbtr/Cdtr: name (Max140Text) and, when idRequired, a CPF
// in PrvtId/Othr/Id or a CNPJ in OrgId/Othr/Id.
func (c *Checker) Party(path string, p xsd.Party, nameRequired, idRequired bool) {
	c.Text(path+"/Nm", p.Nm, 140, nameRequired)
	if p.Id == nil {
		if idRequired {
			c.Addf(path+"/Id", "is required (CPF or CNPJ)")
		}
		return
	}
	switch {
	case p.Id.OrgId != nil && p.Id.PrvtId != nil:
		c.Addf(path+"/Id", "must contain exactly one of OrgId or PrvtId, got both")
	case p.Id.PrvtId != nil:
		c.otherIDs(path+"/Id/PrvtId", p.Id.PrvtId, "CPF", ids.ValidCPF)
	case p.Id.OrgId != nil:
		c.otherIDs(path+"/Id/OrgId", p.Id.OrgId, "CNPJ", ids.ValidCNPJ)
	default:
		c.Addf(path+"/Id", "must contain OrgId or PrvtId")
	}
}

func (c *Checker) otherIDs(path string, o *xsd.OtherIDs, kind string, valid func(string) bool) {
	if len(o.Othr) == 0 {
		c.Addf(path+"/Othr", "is required (%s)", kind)
		return
	}
	for i, g := range o.Othr {
		p := Idx(path+"/Othr", i)
		c.Choice(p+"/SchmeNm", g.SchmeNm, 35)
		c.Max35(p+"/Issr", g.Issr, false)
		if i == 0 {
			if !valid(g.Id) {
				c.Addf(p+"/Id", "must be a valid %s, got %q", kind, g.Id)
			}
			continue
		}
		c.Max35(p+"/Id", g.Id, true)
	}
}

// accountTypes are the Pix account types.
var accountTypes = []string{xsd.AccountCurrent, xsd.AccountSavings, xsd.AccountSalary, xsd.AccountPayments}

// Account checks a Pix account: number (Id/Othr/Id, Max34Text) and type
// are required, the optional Prxy/Id must be a valid Pix key.
func (c *Checker) Account(path string, a *xsd.Account, required bool) {
	if a == nil {
		if required {
			c.Addf(path, "is required")
		}
		return
	}
	switch {
	case a.Id.IBAN != "" && a.Id.Othr != nil:
		c.Addf(path+"/Id", "must contain exactly one of IBAN or Othr, got both")
	case a.Id.IBAN != "":
		c.Text(path+"/Id/IBAN", a.Id.IBAN, 34, true)
	case a.Id.Othr == nil:
		c.Addf(path+"/Id/Othr/Id", "is required (account number)")
	default:
		o := a.Id.Othr
		c.Text(path+"/Id/Othr/Id", o.Id, 34, true)
		c.Choice(path+"/Id/Othr/SchmeNm", o.SchmeNm, 35)
		c.Max35(path+"/Id/Othr/Issr", o.Issr, false)
	}
	if a.Tp == nil {
		c.Addf(path+"/Tp", "is required (CACC, SVGS, SLRY or TRAN)")
	} else {
		c.Choice(path+"/Tp", a.Tp, 35)
		if (a.Tp.Cd == "") != (a.Tp.Prtry == "") {
			c.OneOf(path+"/Tp/"+tpField(a.Tp), a.Tp.Value(), false, accountTypes...)
		}
	}
	if a.Ccy != "" && a.Ccy != xsd.CurrencyBRL {
		c.Addf(path+"/Ccy", "must be %q, got %q", xsd.CurrencyBRL, a.Ccy)
	}
	c.Text(path+"/Nm", a.Nm, 70, false)
	if a.Prxy != nil {
		c.Choice(path+"/Prxy/Tp", a.Prxy.Tp, 35)
		if a.Prxy.Id == "" {
			c.Addf(path+"/Prxy/Id", "is required")
		} else if _, err := ids.ClassifyPixKey(a.Prxy.Id); err != nil {
			c.Addf(path+"/Prxy/Id", "%s", trimPrefix(err.Error(), ids.ErrInvalidPixKey.Error()+": "))
		}
	}
}

func tpField(cp *xsd.CodeOrProprietary) string {
	if cp.Cd != "" {
		return "Cd"
	}
	return "Prtry"
}

// Remittance checks RmtInf/Ustrd (Max140Text each).
func (c *Checker) Remittance(path string, r *xsd.RemittanceInformation) {
	if r == nil {
		return
	}
	for i, u := range r.Ustrd {
		c.Text(Idx(path+"/Ustrd", i), u, 140, true)
	}
}

// NbOfTxs checks a Max15NumericText transaction count against the actual
// number of transactions.
func (c *Checker) NbOfTxs(path, s string, actual int) {
	if s == "" {
		c.Addf(path, "is required")
		return
	}
	n, err := strconv.Atoi(s)
	if err != nil || len(s) > 15 || n < 0 || s[0] == '+' || s[0] == '-' {
		c.Addf(path, "must be a number of up to 15 digits, got %q", s)
		return
	}
	if n != actual {
		c.Addf(path, "is %d but the message has %d transactions", n, actual)
	}
}

func trimPrefix(s, p string) string {
	if len(s) >= len(p) && s[:len(p)] == p {
		return s[len(p):]
	}
	return s
}
