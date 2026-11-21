// Package pacs008 models the ISO 20022 FIToFICustomerCreditTransfer
// message (pacs.008), which carries a Pix payment between participants
// through the SPI.
//
// The structs cover the elements Pix uses and follow the element order of
// pacs.008.001.08, so marshalled output is schema-ordered. Parse accepts
// any pacs.008.001.xx namespace; elements that are not modelled are
// ignored. The BCB SPI catalog further restricts the message (allowed
// elements, code lists, lengths); Validate enforces the ISO 20022 rules
// and the Pix rules this package is confident about, and is not a
// substitute for validating against the current catalog.
package pacs008

import (
	"encoding/xml"

	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// DefaultNamespace is the namespace used when marshalling a Document whose
// XMLName is empty.
const DefaultNamespace = xmlutil.NamespacePrefix + "pacs.008.001.08"

// Pix-wide constants.
const (
	SettlementClearing = "CLRG" // GrpHdr/SttlmInf/SttlmMtd
	ChargesSLEV        = "SLEV" // ChrgBr: charges per service level
)

// Ways a Pix payment was initiated, carried in PmtTpInf/LclInstrm/Prtry.
// The list is informational; Validate accepts any value of up to 35
// characters because the BCB catalog evolves (Pix Automático, for
// instance, added new values).
const (
	InitiationManual       = "MANU" // account details typed by the payer
	InitiationDICT         = "DICT" // Pix key resolved through the DICT
	InitiationStaticQR     = "QRES" // static QR code
	InitiationDynamicQR    = "QRDN" // dynamic QR code
	InitiationPaymentInitr = "INIC" // payment initiation service provider
)

// Document is a pacs.008 message.
type Document struct {
	XMLName           xml.Name                     // Document, in the pacs.008 namespace
	FIToFICstmrCdtTrf FIToFICustomerCreditTransfer `xml:"FIToFICstmrCdtTrf"`
}

// FIToFICustomerCreditTransfer is the message root.
type FIToFICustomerCreditTransfer struct {
	GrpHdr      GroupHeader                 `xml:"GrpHdr"`
	CdtTrfTxInf []CreditTransferTransaction `xml:"CdtTrfTxInf"`
}

// GroupHeader is GrpHdr.
type GroupHeader struct {
	MsgId    string                `xml:"MsgId"`
	CreDtTm  xsd.DateTime          `xml:"CreDtTm"`
	NbOfTxs  string                `xml:"NbOfTxs"`
	SttlmInf SettlementInstruction `xml:"SttlmInf"`
}

// SettlementInstruction is GrpHdr/SttlmInf.
type SettlementInstruction struct {
	SttlmMtd string `xml:"SttlmMtd"`
}

// CreditTransferTransaction is one CdtTrfTxInf: a single Pix payment.
type CreditTransferTransaction struct {
	PmtId          PaymentIdentification      `xml:"PmtId"`
	PmtTpInf       *PaymentTypeInformation    `xml:"PmtTpInf,omitempty"`
	IntrBkSttlmAmt xsd.Amount                 `xml:"IntrBkSttlmAmt"`
	AccptncDtTm    xsd.DateTime               `xml:"AccptncDtTm,omitempty"`
	ChrgBr         string                     `xml:"ChrgBr"`
	Dbtr           xsd.Party                  `xml:"Dbtr"`
	DbtrAcct       *xsd.Account               `xml:"DbtrAcct,omitempty"`
	DbtrAgt        xsd.Agent                  `xml:"DbtrAgt"`
	CdtrAgt        xsd.Agent                  `xml:"CdtrAgt"`
	Cdtr           xsd.Party                  `xml:"Cdtr"`
	CdtrAcct       *xsd.Account               `xml:"CdtrAcct,omitempty"`
	Purp           *xsd.CodeOrProprietary     `xml:"Purp,omitempty"`
	RmtInf         *xsd.RemittanceInformation `xml:"RmtInf,omitempty"`
}

// PaymentIdentification is PmtId. TxId carries the transaction identifier
// of a QR code charge (txid), when there is one.
type PaymentIdentification struct {
	EndToEndId string `xml:"EndToEndId"`
	TxId       string `xml:"TxId,omitempty"`
}

// PaymentTypeInformation is PmtTpInf.
type PaymentTypeInformation struct {
	InstrPrty string                 `xml:"InstrPrty,omitempty"`
	LclInstrm *xsd.CodeOrProprietary `xml:"LclInstrm,omitempty"`
	CtgyPurp  *xsd.CodeOrProprietary `xml:"CtgyPurp,omitempty"`
}

// Amount returns the interbank settlement amount.
func (t *CreditTransferTransaction) Amount() money.Amount { return t.IntrBkSttlmAmt.Value }

// Initiation returns PmtTpInf/LclInstrm/Prtry (or Cd), or "".
func (t *CreditTransferTransaction) Initiation() string {
	if t.PmtTpInf == nil {
		return ""
	}
	return t.PmtTpInf.LclInstrm.Value()
}

// RemittanceInfo returns the unstructured remittance lines joined by
// newlines.
func (t *CreditTransferTransaction) RemittanceInfo() string {
	if t.RmtInf == nil {
		return ""
	}
	s := ""
	for i, u := range t.RmtInf.Ustrd {
		if i > 0 {
			s += "\n"
		}
		s += u
	}
	return s
}

// Transactions returns the CdtTrfTxInf entries.
func (d *Document) Transactions() []CreditTransferTransaction { return d.FIToFICstmrCdtTrf.CdtTrfTxInf }

// Version returns the message version from the namespace, e.g. "08", or ""
// if the document has no namespace yet.
func (d *Document) Version() string {
	id, ok := xmlutil.ParseMessageID(d.XMLName.Space)
	if !ok {
		return ""
	}
	return id.Version
}

// MessageName returns the full message identifier, e.g. "pacs.008.001.08",
// using DefaultNamespace when the document has no namespace.
func (d *Document) MessageName() string {
	id, _ := xmlutil.ParseMessageID(xmlutil.Name(d.XMLName, DefaultNamespace).Space)
	return id.String()
}

// Parse decodes a pacs.008 message, either a bare Document or a business
// message that wraps an AppHdr and a Document. It checks well-formedness,
// the namespace (pacs.008, any version) and the lexical form of amounts;
// call Validate for the business rules.
func Parse(data []byte) (*Document, error) {
	var d Document
	if _, err := xmlutil.DecodeDocument(data, "pacs.008", &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Marshal renders the document as indented XML with an XML declaration.
// It does not validate; call Validate first.
func (d *Document) Marshal() ([]byte, error) {
	c := *d
	c.XMLName = xmlutil.Name(d.XMLName, DefaultNamespace)
	return xmlutil.Marshal(&c)
}
