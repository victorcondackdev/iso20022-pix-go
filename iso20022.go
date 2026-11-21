// Package iso20022 detects and inspects ISO 20022 messages exchanged in
// Brazil's instant payment system, Pix, through the SPI (Sistema de
// Pagamentos Instantâneos).
//
// Detect reads only as far as the Document start element, with a
// streaming xml.Decoder, to tell which message a payload carries
// (pacs.008, pacs.002, pacs.004, camt.054, any version) before choosing a
// parser. Inspect additionally decodes the business application header
// (head.001 AppHdr) when one is present.
//
// The message models live in subpackages:
//
//   - pacs008: FIToFICustomerCreditTransfer (a Pix payment)
//   - pacs002: FIToFIPaymentStatusReport (acceptance or rejection)
//   - pacs004: PaymentReturn (devolução, including MED)
//   - camt054: BankToCustomerDebitCreditNotification
//   - ids:     EndToEndId, RtrId, MsgId, ISPB, CPF, CNPJ and Pix keys
//   - money:   exact BRL amounts in centavos
//
// This module is not affiliated with the Banco Central do Brasil. The SPI
// catalogs published by the BCB further constrain every message; validate
// production traffic against the current catalog.
package iso20022

import (
	"bytes"
	"io"

	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
)

// Errors returned by Detect, Inspect and the subpackage Parse functions.
// They can be matched with errors.Is.
var (
	ErrNoDocument         = xmlutil.ErrNoDocument
	ErrUnknownNamespace   = xmlutil.ErrUnknownNamespace
	ErrUnexpectedMessage  = xmlutil.ErrUnexpectedMessage
	ErrDirectiveForbidden = xmlutil.ErrDirectiveForbidden
	ErrMultipleDocuments  = xmlutil.ErrMultipleDocuments
)

// Message identifiers without version, as returned by MessageType.ID.
const (
	Pacs008 = "pacs.008"
	Pacs002 = "pacs.002"
	Pacs004 = "pacs.004"
	Camt054 = "camt.054"
	Head001 = "head.001"
)

var names = map[string]string{
	Pacs008: "FIToFICustomerCreditTransfer",
	Pacs002: "FIToFIPaymentStatusReport",
	Pacs004: "PaymentReturn",
	Camt054: "BankToCustomerDebitCreditNotification",
	Head001: "BusinessApplicationHeader",
}

// MessageType identifies an ISO 20022 message definition, such as
// pacs.008.001.08: business area "pacs", message functionality "008",
// variant "001" and version "08".
type MessageType struct {
	BusinessArea  string
	Functionality string
	Variant       string
	Version       string
}

// ParseMessageType parses "pacs.008.001.08" or the corresponding
// namespace "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08".
func ParseMessageType(s string) (MessageType, error) {
	id, ok := xmlutil.ParseMessageID(s)
	if !ok {
		return MessageType{}, ErrUnknownNamespace
	}
	return MessageType{id.BusinessArea, id.Function, id.Variant, id.Version}, nil
}

// String returns the full identifier, e.g. "pacs.008.001.08".
func (m MessageType) String() string {
	if m.BusinessArea == "" {
		return ""
	}
	return m.BusinessArea + "." + m.Functionality + "." + m.Variant + "." + m.Version
}

// ID returns the identifier without variant and version, e.g. "pacs.008",
// for comparison with the Pacs008, Pacs002, ... constants.
func (m MessageType) ID() string {
	if m.BusinessArea == "" {
		return ""
	}
	return m.BusinessArea + "." + m.Functionality
}

// Namespace returns the XML namespace of the message.
func (m MessageType) Namespace() string { return xmlutil.NamespacePrefix + m.String() }

// Name returns the ISO 20022 message name, e.g.
// "FIToFICustomerCreditTransfer", or "" for messages this module does not
// model.
func (m MessageType) Name() string { return names[m.ID()] }

// Supported reports whether a subpackage of this module parses the
// message.
func (m MessageType) Supported() bool {
	switch m.ID() {
	case Pacs008, Pacs002, Pacs004, Camt054:
		return true
	}
	return false
}

// Detect returns the type of the ISO 20022 message in data. It scans
// tokens only up to the Document start element; the message body is not
// decoded, so detection cost does not depend on message size. Any wrapper
// element and an AppHdr before the Document are skipped. XML directives
// such as DOCTYPE are rejected.
func Detect(data []byte) (MessageType, error) {
	return DetectReader(bytes.NewReader(data))
}

// DetectReader is Detect for a stream. It stops reading right after the
// Document start tag.
func DetectReader(r io.Reader) (MessageType, error) {
	s := xmlutil.NewScanner(r)
	for {
		se, err := s.Next()
		if err == io.EOF {
			return MessageType{}, ErrNoDocument
		}
		if err != nil {
			return MessageType{}, err
		}
		if se.Name.Local == "AppHdr" {
			if err := s.D.Skip(); err != nil {
				return MessageType{}, err
			}
			continue
		}
		if xmlutil.IsDocument(se) {
			id, err := xmlutil.DocumentID(se)
			if err != nil {
				return MessageType{}, err
			}
			return MessageType{id.BusinessArea, id.Function, id.Variant, id.Version}, nil
		}
	}
}
