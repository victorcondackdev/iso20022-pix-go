// Package xsd holds the ISO 20022 building blocks shared by the message
// packages: date and time types, amounts, parties, accounts, financial
// institution identifications and the validation error type.
//
// Struct field names mirror the ISO 20022 XML tags (MsgId, Dbtr, Othr, ...)
// so that the Go code reads like the message definition report; every
// field carries the exact XML element name in its struct tag. Only the
// elements that Pix messages use are modelled. When a message is parsed,
// elements that are not modelled are ignored.
package xsd

import (
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/money"
)

// CurrencyBRL is the ISO 4217 code of the Brazilian real, the only
// currency settled by the SPI.
const CurrencyBRL = "BRL"

// DateTime is an ISO 20022 ISODateTime kept in its lexical form, so that
// parsing and re-marshalling a message never changes it. Use NewDateTime
// to build one and Time to interpret it.
type DateTime string

// dateTimeLayout is the form produced by NewDateTime: UTC with
// millisecond precision, as commonly used by the SPI.
const dateTimeLayout = "2006-01-02T15:04:05.000Z"

// NewDateTime formats t in UTC with millisecond precision, for example
// "2026-11-21T10:30:00.000Z".
func NewDateTime(t time.Time) DateTime {
	return DateTime(t.UTC().Format(dateTimeLayout))
}

// Time parses the value. ISODateTime allows an optional fraction of a
// second and an optional "Z" or ±hh:mm offset; a value without an offset
// is local time of unspecified zone and is returned as UTC.
func (d DateTime) Time() (time.Time, error) {
	s := string(d)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02T15:04:05.999999999", s)
}

// Date is an ISO 20022 ISODate (YYYY-MM-DD) kept in its lexical form.
type Date string

// NewDate formats the calendar date of t in t's location.
func NewDate(t time.Time) Date { return Date(t.Format(time.DateOnly)) }

// Time parses the date as midnight UTC.
func (d Date) Time() (time.Time, error) { return time.Parse(time.DateOnly, string(d)) }

// DateAndDateTime is the DateAndDateTime2Choice component: exactly one of
// Dt or DtTm.
type DateAndDateTime struct {
	Dt   Date     `xml:"Dt,omitempty"`
	DtTm DateTime `xml:"DtTm,omitempty"`
}

// Amount is the ActiveCurrencyAndAmount (or ActiveOrHistoric...) component:
// a decimal amount with a Ccy attribute.
type Amount struct {
	Ccy   string       `xml:"Ccy,attr"`
	Value money.Amount `xml:",chardata"`
}

// BRL returns an amount in Brazilian reais.
func BRL(a money.Amount) Amount { return Amount{Ccy: CurrencyBRL, Value: a} }

// String formats the amount as "10.50 BRL".
func (a Amount) String() string { return a.Value.String() + " " + a.Ccy }

// CodeOrProprietary models the many ISO 20022 choice components made of
// an external code (Cd) or a proprietary value (Prtry), such as
// LclInstrm, CtgyPurp, Purp, account Tp and reason Rsn. Exactly one of the
// two must be present.
type CodeOrProprietary struct {
	Cd    string `xml:"Cd,omitempty"`
	Prtry string `xml:"Prtry,omitempty"`
}

// Value returns Cd if set, otherwise Prtry.
func (c *CodeOrProprietary) Value() string {
	if c == nil {
		return ""
	}
	if c.Cd != "" {
		return c.Cd
	}
	return c.Prtry
}

// Code returns a CodeOrProprietary holding an external code.
func Code(cd string) *CodeOrProprietary { return &CodeOrProprietary{Cd: cd} }

// Proprietary returns a CodeOrProprietary holding a proprietary value.
func Proprietary(p string) *CodeOrProprietary { return &CodeOrProprietary{Prtry: p} }

// Party is the PartyIdentification component used for Dbtr and Cdtr.
type Party struct {
	Nm string   `xml:"Nm,omitempty"`
	Id *PartyID `xml:"Id,omitempty"`
}

// PartyID is the Party38Choice component: an organisation (OrgId) or a
// private person (PrvtId).
type PartyID struct {
	OrgId  *OtherIDs `xml:"OrgId,omitempty"`
	PrvtId *OtherIDs `xml:"PrvtId,omitempty"`
}

// OtherIDs holds the Othr identifications of an OrgId or PrvtId. In Pix a
// person is identified by CPF and an organisation by CNPJ, both in
// Othr/Id.
type OtherIDs struct {
	Othr []GenericID `xml:"Othr"`
}

// GenericID is the GenericOrganisationIdentification /
// GenericPersonIdentification component.
type GenericID struct {
	Id      string             `xml:"Id"`
	SchmeNm *CodeOrProprietary `xml:"SchmeNm,omitempty"`
	Issr    string             `xml:"Issr,omitempty"`
}

// Person returns a party identified by CPF (PrvtId/Othr/Id).
func Person(name, cpf string) Party {
	return Party{Nm: name, Id: &PartyID{PrvtId: &OtherIDs{Othr: []GenericID{{Id: cpf}}}}}
}

// Organisation returns a party identified by CNPJ (OrgId/Othr/Id).
func Organisation(name, cnpj string) Party {
	return Party{Nm: name, Id: &PartyID{OrgId: &OtherIDs{Othr: []GenericID{{Id: cnpj}}}}}
}

// TaxID returns the first Othr/Id of the party's PrvtId or OrgId, which in
// Pix is the CPF or CNPJ, or "" if there is none.
func (p Party) TaxID() string {
	if p.Id == nil {
		return ""
	}
	for _, o := range []*OtherIDs{p.Id.PrvtId, p.Id.OrgId} {
		if o != nil && len(o.Othr) > 0 {
			return o.Othr[0].Id
		}
	}
	return ""
}

// Account is the CashAccount component used for DbtrAcct, CdtrAcct and the
// camt.054 Acct.
type Account struct {
	Id   AccountID          `xml:"Id"`
	Tp   *CodeOrProprietary `xml:"Tp,omitempty"`
	Ccy  string             `xml:"Ccy,omitempty"`
	Nm   string             `xml:"Nm,omitempty"`
	Prxy *Proxy             `xml:"Prxy,omitempty"`
}

// AccountID is the AccountIdentification4Choice component. Pix uses Othr.
type AccountID struct {
	IBAN string            `xml:"IBAN,omitempty"`
	Othr *GenericAccountID `xml:"Othr,omitempty"`
}

// GenericAccountID is the GenericAccountIdentification1 component. In Pix,
// Id is the account number and Issr the branch (agência).
type GenericAccountID struct {
	Id      string             `xml:"Id"`
	SchmeNm *CodeOrProprietary `xml:"SchmeNm,omitempty"`
	Issr    string             `xml:"Issr,omitempty"`
}

// Proxy is the ProxyAccountIdentification1 component (Prxy). In Pix, Id is
// the Pix key used to address the creditor account.
type Proxy struct {
	Tp *CodeOrProprietary `xml:"Tp,omitempty"`
	Id string             `xml:"Id"`
}

// Pix account types, carried in Tp/Prtry.
const (
	AccountCurrent  = "CACC" // conta corrente
	AccountSavings  = "SVGS" // conta poupança
	AccountSalary   = "SLRY" // conta salário
	AccountPayments = "TRAN" // conta de pagamento
)

// NewAccount returns a Pix account with the given branch (Issr, may be
// empty for payment accounts), number and type (Tp/Prtry).
func NewAccount(branch, number, accountType string) *Account {
	return &Account{
		Id: AccountID{Othr: &GenericAccountID{Id: number, Issr: branch}},
		Tp: Proprietary(accountType),
	}
}

// Number returns Id/Othr/Id, or "" if absent.
func (a *Account) Number() string {
	if a == nil || a.Id.Othr == nil {
		return ""
	}
	return a.Id.Othr.Id
}

// Branch returns Id/Othr/Issr, or "" if absent.
func (a *Account) Branch() string {
	if a == nil || a.Id.Othr == nil {
		return ""
	}
	return a.Id.Othr.Issr
}

// PixKey returns Prxy/Id, or "" if absent.
func (a *Account) PixKey() string {
	if a == nil || a.Prxy == nil {
		return ""
	}
	return a.Prxy.Id
}

// Agent is the BranchAndFinancialInstitutionIdentification component used
// for DbtrAgt, CdtrAgt and the head.001 FIId. Pix identifies participants
// by ISPB in FinInstnId/ClrSysMmbId/MmbId.
type Agent struct {
	FinInstnId FinancialInstitutionID `xml:"FinInstnId"`
}

// FinancialInstitutionID is the FinancialInstitutionIdentification
// component.
type FinancialInstitutionID struct {
	ClrSysMmbId *ClearingSystemMemberID `xml:"ClrSysMmbId,omitempty"`
}

// ClearingSystemMemberID is the ClearingSystemMemberIdentification2
// component.
type ClearingSystemMemberID struct {
	ClrSysId *CodeOrProprietary `xml:"ClrSysId,omitempty"`
	MmbId    string             `xml:"MmbId"`
}

// AgentISPB returns an agent identified by ISPB.
func AgentISPB(ispb string) Agent {
	return Agent{FinInstnId: FinancialInstitutionID{ClrSysMmbId: &ClearingSystemMemberID{MmbId: ispb}}}
}

// ISPB returns FinInstnId/ClrSysMmbId/MmbId, or "" if absent.
func (a Agent) ISPB() string {
	if a.FinInstnId.ClrSysMmbId == nil {
		return ""
	}
	return a.FinInstnId.ClrSysMmbId.MmbId
}

// RemittanceInformation is the RmtInf component. Pix uses unstructured
// remittance information (Ustrd), the free-text message shown to the
// payee.
type RemittanceInformation struct {
	Ustrd []string `xml:"Ustrd"`
}
