// Package pacs004 models the ISO 20022 PaymentReturn message (pacs.004),
// used for Pix returns (devoluções), including returns requested through
// the Mecanismo Especial de Devolução (MED).
//
// The structs follow the element order of pacs.004.001.09 and Parse
// accepts any pacs.004.001.xx namespace. FromPacs008 builds a return from
// the original payment and refuses to return more than was paid.
package pacs004

import (
	"encoding/xml"

	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// DefaultNamespace is the namespace used when marshalling a Document whose
// XMLName is empty.
const DefaultNamespace = xmlutil.NamespacePrefix + "pacs.004.001.09"

// Document is a pacs.004 message.
type Document struct {
	XMLName xml.Name      // Document, in the pacs.004 namespace
	PmtRtr  PaymentReturn `xml:"PmtRtr"`
}

// PaymentReturn is the message root.
type PaymentReturn struct {
	GrpHdr GroupHeader         `xml:"GrpHdr"`
	TxInf  []ReturnTransaction `xml:"TxInf"`
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

// ReturnTransaction is one TxInf.
type ReturnTransaction struct {
	RtrId               string                    `xml:"RtrId"`
	OrgnlGrpInf         *OriginalGroupInformation `xml:"OrgnlGrpInf,omitempty"`
	OrgnlEndToEndId     string                    `xml:"OrgnlEndToEndId"`
	OrgnlTxId           string                    `xml:"OrgnlTxId,omitempty"`
	OrgnlIntrBkSttlmAmt *xsd.Amount               `xml:"OrgnlIntrBkSttlmAmt,omitempty"`
	RtrdIntrBkSttlmAmt  xsd.Amount                `xml:"RtrdIntrBkSttlmAmt"`
	ChrgBr              string                    `xml:"ChrgBr,omitempty"`
	RtrRsnInf           []ReturnReasonInfo        `xml:"RtrRsnInf,omitempty"`
}

// OriginalGroupInformation is TxInf/OrgnlGrpInf.
type OriginalGroupInformation struct {
	OrgnlMsgId   string       `xml:"OrgnlMsgId"`
	OrgnlMsgNmId string       `xml:"OrgnlMsgNmId"`
	OrgnlCreDtTm xsd.DateTime `xml:"OrgnlCreDtTm,omitempty"`
}

// ReturnReasonInfo is RtrRsnInf.
type ReturnReasonInfo struct {
	Rsn      *xsd.CodeOrProprietary `xml:"Rsn,omitempty"`
	AddtlInf []string               `xml:"AddtlInf,omitempty"`
}

// Amount returns the returned amount.
func (t *ReturnTransaction) Amount() money.Amount { return t.RtrdIntrBkSttlmAmt.Value }

// Reason returns the first return reason code, or "".
func (t *ReturnTransaction) Reason() string {
	for _, r := range t.RtrRsnInf {
		if v := r.Rsn.Value(); v != "" {
			return v
		}
	}
	return ""
}

// Transactions returns the TxInf entries.
func (d *Document) Transactions() []ReturnTransaction { return d.PmtRtr.TxInf }

// Version returns the version from the namespace, e.g. "09", or "".
func (d *Document) Version() string {
	id, _ := xmlutil.ParseMessageID(d.XMLName.Space)
	return id.Version
}

// Parse decodes a pacs.004 message (bare Document or business message).
func Parse(data []byte) (*Document, error) {
	var d Document
	if _, err := xmlutil.DecodeDocument(data, "pacs.004", &d); err != nil {
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
