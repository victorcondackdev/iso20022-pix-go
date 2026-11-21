// Package camt054 models the ISO 20022 BankToCustomerDebitCreditNotification
// message (camt.054): notifications of entries booked to an account, such
// as the credits and debits of Pix payments and returns, with the
// EndToEndId of each underlying transaction.
//
// The structs follow the element order of camt.054.001.08 and Parse
// accepts any camt.054.001.xx namespace.
package camt054

import (
	"encoding/xml"

	"github.com/victorcondackdev/iso20022-pix-go/internal/check"
	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// DefaultNamespace is the namespace used when marshalling a Document whose
// XMLName is empty.
const DefaultNamespace = xmlutil.NamespacePrefix + "camt.054.001.08"

// Credit/debit indicators (CdtDbtInd).
const (
	Credit = "CRDT"
	Debit  = "DBIT"
)

// Entry status codes (Sts/Cd, ExternalEntryStatus1Code).
const (
	StatusBooked  = "BOOK"
	StatusPending = "PDNG"
	StatusInfo    = "INFO"
)

// Document is a camt.054 message.
type Document struct {
	XMLName               xml.Name     // Document, in the camt.054 namespace
	BkToCstmrDbtCdtNtfctn Notification `xml:"BkToCstmrDbtCdtNtfctn"`
}

// Notification is the message root.
type Notification struct {
	GrpHdr GroupHeader           `xml:"GrpHdr"`
	Ntfctn []AccountNotification `xml:"Ntfctn"`
}

// GroupHeader is GrpHdr.
type GroupHeader struct {
	MsgId   string       `xml:"MsgId"`
	CreDtTm xsd.DateTime `xml:"CreDtTm"`
}

// AccountNotification is one Ntfctn: the entries of one account.
type AccountNotification struct {
	Id      string       `xml:"Id"`
	CreDtTm xsd.DateTime `xml:"CreDtTm,omitempty"`
	Acct    xsd.Account  `xml:"Acct"`
	Ntry    []Entry      `xml:"Ntry"`
}

// Entry is one Ntry.
type Entry struct {
	NtryRef      string                `xml:"NtryRef,omitempty"`
	Amt          xsd.Amount            `xml:"Amt"`
	CdtDbtInd    string                `xml:"CdtDbtInd"`
	RvslInd      *bool                 `xml:"RvslInd,omitempty"`
	Sts          xsd.CodeOrProprietary `xml:"Sts"`
	BookgDt      *xsd.DateAndDateTime  `xml:"BookgDt,omitempty"`
	ValDt        *xsd.DateAndDateTime  `xml:"ValDt,omitempty"`
	AcctSvcrRef  string                `xml:"AcctSvcrRef,omitempty"`
	BkTxCd       BankTransactionCode   `xml:"BkTxCd"`
	NtryDtls     []EntryDetails        `xml:"NtryDtls,omitempty"`
	AddtlNtryInf string                `xml:"AddtlNtryInf,omitempty"`
}

// BankTransactionCode is BkTxCd: a structured domain code, a proprietary
// code, or both.
type BankTransactionCode struct {
	Domn  *Domain                `xml:"Domn,omitempty"`
	Prtry *ProprietaryBankTxCode `xml:"Prtry,omitempty"`
}

// Domain is BkTxCd/Domn.
type Domain struct {
	Cd   string `xml:"Cd"`
	Fmly Family `xml:"Fmly"`
}

// Family is BkTxCd/Domn/Fmly.
type Family struct {
	Cd        string `xml:"Cd"`
	SubFmlyCd string `xml:"SubFmlyCd"`
}

// ProprietaryBankTxCode is BkTxCd/Prtry.
type ProprietaryBankTxCode struct {
	Cd   string `xml:"Cd"`
	Issr string `xml:"Issr,omitempty"`
}

// EntryDetails is NtryDtls.
type EntryDetails struct {
	TxDtls []TransactionDetails `xml:"TxDtls"`
}

// TransactionDetails is NtryDtls/TxDtls.
type TransactionDetails struct {
	Refs       *References                `xml:"Refs,omitempty"`
	Amt        *xsd.Amount                `xml:"Amt,omitempty"`
	CdtDbtInd  string                     `xml:"CdtDbtInd,omitempty"`
	RmtInf     *xsd.RemittanceInformation `xml:"RmtInf,omitempty"`
	AddtlTxInf string                     `xml:"AddtlTxInf,omitempty"`
}

// References is TxDtls/Refs.
type References struct {
	MsgId       string `xml:"MsgId,omitempty"`
	AcctSvcrRef string `xml:"AcctSvcrRef,omitempty"`
	InstrId     string `xml:"InstrId,omitempty"`
	EndToEndId  string `xml:"EndToEndId,omitempty"`
	TxId        string `xml:"TxId,omitempty"`
}

// EndToEndIDs returns the EndToEndId of every transaction detail of the
// entry, in order, skipping details without one.
func (e *Entry) EndToEndIDs() []string {
	var out []string
	for _, d := range e.NtryDtls {
		for _, t := range d.TxDtls {
			if t.Refs != nil && t.Refs.EndToEndId != "" {
				out = append(out, t.Refs.EndToEndId)
			}
		}
	}
	return out
}

// Signed returns the entry amount, negative for debits.
func (e *Entry) Signed() money.Amount {
	if e.CdtDbtInd == Debit {
		return -e.Amt.Value
	}
	return e.Amt.Value
}

// Entries returns the entries of every notification, in order.
func (d *Document) Entries() []Entry {
	var out []Entry
	for _, n := range d.BkToCstmrDbtCdtNtfctn.Ntfctn {
		out = append(out, n.Ntry...)
	}
	return out
}

// Version returns the version from the namespace, e.g. "08", or "".
func (d *Document) Version() string {
	id, _ := xmlutil.ParseMessageID(d.XMLName.Space)
	return id.Version
}

// Parse decodes a camt.054 message (bare Document or business message).
func Parse(data []byte) (*Document, error) {
	var d Document
	if _, err := xmlutil.DecodeDocument(data, "camt.054", &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Marshal renders the document as indented XML. It does not validate.
func (d *Document) Marshal() ([]byte, error) {
	c := *d
	c.XMLName = xmlutil.Name(d.XMLName, DefaultNamespace)
	return xmlutil.Marshal(&c)
}

const root = "/Document/BkToCstmrDbtCdtNtfctn"

// Validate checks the notification and returns every violation as
// xsd.Violations, or nil: MsgId and CreDtTm, at least one Ntfctn with an
// Id and an account number, and for each entry a BRL amount, CdtDbtInd
// CRDT or DBIT, a status, a booking date when the status is BOOK, a bank
// transaction code, well-formed references (an EndToEndId must be a valid
// Pix EndToEndId) and, when every transaction detail carries an amount,
// details that add up to the entry amount.
func (d *Document) Validate() error {
	var c check.Checker
	if d.XMLName.Space != "" {
		if id, ok := xmlutil.ParseMessageID(d.XMLName.Space); !ok || id.Base() != "camt.054" {
			c.Addf("/Document/@xmlns", "must be a camt.054 namespace, got %q", d.XMLName.Space)
		}
	}
	m := &d.BkToCstmrDbtCdtNtfctn
	c.Max35(root+"/GrpHdr/MsgId", m.GrpHdr.MsgId, true)
	c.DateTime(root+"/GrpHdr/CreDtTm", m.GrpHdr.CreDtTm, true)
	if len(m.Ntfctn) == 0 {
		c.Addf(root+"/Ntfctn", "at least one notification is required")
	}
	for i := range m.Ntfctn {
		n := &m.Ntfctn[i]
		p := check.Idx(root+"/Ntfctn", i)
		c.Max35(p+"/Id", n.Id, true)
		c.DateTime(p+"/CreDtTm", n.CreDtTm, false)
		switch {
		case n.Acct.Id.Othr != nil:
			c.Text(p+"/Acct/Id/Othr/Id", n.Acct.Id.Othr.Id, 34, true)
			c.Max35(p+"/Acct/Id/Othr/Issr", n.Acct.Id.Othr.Issr, false)
		case n.Acct.Id.IBAN != "":
			c.Text(p+"/Acct/Id/IBAN", n.Acct.Id.IBAN, 34, true)
		default:
			c.Addf(p+"/Acct/Id", "is required")
		}
		for j := range n.Ntry {
			validateEntry(&c, check.Idx(p+"/Ntry", j), &n.Ntry[j])
		}
	}
	return c.Err()
}

func validateEntry(c *check.Checker, p string, e *Entry) {
	c.Max35(p+"/NtryRef", e.NtryRef, false)
	c.BRL(p+"/Amt", e.Amt, false)
	c.OneOf(p+"/CdtDbtInd", e.CdtDbtInd, true, Credit, Debit)
	if e.Sts.Cd == "" && e.Sts.Prtry == "" {
		c.Addf(p+"/Sts", "is required")
	} else {
		c.Choice(p+"/Sts", &e.Sts, 35)
	}
	if e.Sts.Cd == StatusBooked && e.BookgDt == nil {
		c.Addf(p+"/BookgDt", "is required for booked entries")
	}
	c.DateOrDateTime(p+"/BookgDt", e.BookgDt, false)
	c.DateOrDateTime(p+"/ValDt", e.ValDt, false)
	c.Max35(p+"/AcctSvcrRef", e.AcctSvcrRef, false)
	switch b := e.BkTxCd; {
	case b.Domn == nil && b.Prtry == nil:
		c.Addf(p+"/BkTxCd", "must contain Domn or Prtry")
	default:
		if b.Domn != nil {
			c.Code(p+"/BkTxCd/Domn/Cd", b.Domn.Cd, true)
			c.Code(p+"/BkTxCd/Domn/Fmly/Cd", b.Domn.Fmly.Cd, true)
			c.Code(p+"/BkTxCd/Domn/Fmly/SubFmlyCd", b.Domn.Fmly.SubFmlyCd, true)
		}
		if b.Prtry != nil {
			c.Max35(p+"/BkTxCd/Prtry/Cd", b.Prtry.Cd, true)
			c.Max35(p+"/BkTxCd/Prtry/Issr", b.Prtry.Issr, false)
		}
	}
	var (
		sum      money.Amount
		allAmts  = true
		nDetails int
	)
	for i, nd := range e.NtryDtls {
		for j, t := range nd.TxDtls {
			nDetails++
			q := check.Idx(check.Idx(p+"/NtryDtls", i)+"/TxDtls", j)
			if r := t.Refs; r != nil {
				c.Max35(q+"/Refs/MsgId", r.MsgId, false)
				c.Max35(q+"/Refs/AcctSvcrRef", r.AcctSvcrRef, false)
				c.Max35(q+"/Refs/InstrId", r.InstrId, false)
				if r.EndToEndId != "" {
					c.EndToEndID(q+"/Refs/EndToEndId", r.EndToEndId)
				}
				c.Max35(q+"/Refs/TxId", r.TxId, false)
			}
			if t.Amt == nil {
				allAmts = false
			} else {
				c.BRL(q+"/Amt", *t.Amt, false)
				if s, err := sum.Add(t.Amt.Value); err == nil {
					sum = s
				} else {
					allAmts = false
				}
			}
			c.OneOf(q+"/CdtDbtInd", t.CdtDbtInd, false, Credit, Debit)
			c.Remittance(q+"/RmtInf", t.RmtInf)
			c.Text(q+"/AddtlTxInf", t.AddtlTxInf, 500, false)
		}
	}
	if nDetails > 0 && allAmts && sum != e.Amt.Value {
		c.Addf(p+"/Amt", "is %s but the transaction details add up to %s", e.Amt.Value, sum)
	}
	c.Text(p+"/AddtlNtryInf", e.AddtlNtryInf, 500, false)
}
