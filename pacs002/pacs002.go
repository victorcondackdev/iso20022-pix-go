// Package pacs002 models the ISO 20022 FIToFIPaymentStatusReport message
// (pacs.002), which reports the outcome of a Pix payment: accepted and
// settled (ACSC) or rejected (RJCT) with a reason code.
//
// The structs follow the element order of pacs.002.001.10 and Parse
// accepts any pacs.002.001.xx namespace. The SPI catalog restricts which
// status and reason codes each participant may send; Reasons lists only
// codes whose ISO 20022 meaning is well established, and unknown codes are
// preserved as-is.
package pacs002

import (
	"encoding/xml"

	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// DefaultNamespace is the namespace used when marshalling a Document whose
// XMLName is empty.
const DefaultNamespace = xmlutil.NamespacePrefix + "pacs.002.001.10"

// Transaction status codes (ExternalPaymentTransactionStatus1Code). Pix
// uses ACSC and RJCT; the others are listed for completeness.
const (
	StatusAcceptedSettlementCompleted = "ACSC" // settled on the debtor side
	StatusRejected                    = "RJCT"
	StatusAcceptedCreditorAccount     = "ACCC" // credited to the creditor account
	StatusAcceptedSettlementInProcess = "ACSP"
	StatusPending                     = "PDNG"
)

// Document is a pacs.002 message.
type Document struct {
	XMLName         xml.Name     // Document, in the pacs.002 namespace
	FIToFIPmtStsRpt StatusReport `xml:"FIToFIPmtStsRpt"`
}

// StatusReport is the message root.
type StatusReport struct {
	GrpHdr            GroupHeader           `xml:"GrpHdr"`
	OrgnlGrpInfAndSts []OriginalGroupHeader `xml:"OrgnlGrpInfAndSts,omitempty"`
	TxInfAndSts       []TransactionStatus   `xml:"TxInfAndSts"`
}

// GroupHeader is GrpHdr.
type GroupHeader struct {
	MsgId   string       `xml:"MsgId"`
	CreDtTm xsd.DateTime `xml:"CreDtTm"`
}

// OriginalGroupHeader is OrgnlGrpInfAndSts: a reference to the message
// being reported on.
type OriginalGroupHeader struct {
	OrgnlMsgId   string       `xml:"OrgnlMsgId"`
	OrgnlMsgNmId string       `xml:"OrgnlMsgNmId"`
	OrgnlCreDtTm xsd.DateTime `xml:"OrgnlCreDtTm,omitempty"`
	GrpSts       string       `xml:"GrpSts,omitempty"`
}

// TransactionStatus is one TxInfAndSts.
type TransactionStatus struct {
	StsId             string               `xml:"StsId,omitempty"`
	OrgnlEndToEndId   string               `xml:"OrgnlEndToEndId"`
	OrgnlTxId         string               `xml:"OrgnlTxId,omitempty"`
	TxSts             string               `xml:"TxSts"`
	StsRsnInf         []StatusReasonInfo   `xml:"StsRsnInf,omitempty"`
	FctvIntrBkSttlmDt *xsd.DateAndDateTime `xml:"FctvIntrBkSttlmDt,omitempty"`
}

// StatusReasonInfo is StsRsnInf.
type StatusReasonInfo struct {
	Rsn      *xsd.CodeOrProprietary `xml:"Rsn,omitempty"`
	AddtlInf []string               `xml:"AddtlInf,omitempty"`
}

// Reason returns the first reason code (Cd or Prtry), or "".
func (t *TransactionStatus) Reason() string {
	for _, r := range t.StsRsnInf {
		if v := r.Rsn.Value(); v != "" {
			return v
		}
	}
	return ""
}

// Accepted reports whether TxSts is ACSC or ACCC.
func (t *TransactionStatus) Accepted() bool {
	return t.TxSts == StatusAcceptedSettlementCompleted || t.TxSts == StatusAcceptedCreditorAccount
}

// Transactions returns the TxInfAndSts entries.
func (d *Document) Transactions() []TransactionStatus { return d.FIToFIPmtStsRpt.TxInfAndSts }

// Version returns the version from the namespace, e.g. "10", or "".
func (d *Document) Version() string {
	id, _ := xmlutil.ParseMessageID(d.XMLName.Space)
	return id.Version
}

// Parse decodes a pacs.002 message (bare Document or business message).
func Parse(data []byte) (*Document, error) {
	var d Document
	if _, err := xmlutil.DecodeDocument(data, "pacs.002", &d); err != nil {
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
