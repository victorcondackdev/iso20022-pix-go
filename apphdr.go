package iso20022

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/internal/check"
	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// AppHdrNamespace is the namespace used when marshalling an AppHdr whose
// XMLName is empty.
const AppHdrNamespace = xmlutil.NamespacePrefix + "head.001.001.01"

// AppHdr is the ISO 20022 business application header (head.001), which
// travels next to the Document and identifies sender, receiver and the
// message definition. Pix participants are identified by ISPB in
// FIId/FinInstnId/ClrSysMmbId/MmbId.
type AppHdr struct {
	XMLName   xml.Name     // AppHdr, in the head.001 namespace
	Fr        HeaderParty  `xml:"Fr"`
	To        HeaderParty  `xml:"To"`
	BizMsgIdr string       `xml:"BizMsgIdr"`
	MsgDefIdr string       `xml:"MsgDefIdr"`
	BizSvc    string       `xml:"BizSvc,omitempty"`
	CreDt     xsd.DateTime `xml:"CreDt"`
}

// HeaderParty is the Fr/To party of the AppHdr, identified as a financial
// institution (FIId).
type HeaderParty struct {
	FIId *xsd.Agent `xml:"FIId,omitempty"`
}

// ISPB returns FIId/FinInstnId/ClrSysMmbId/MmbId, or "".
func (p HeaderParty) ISPB() string {
	if p.FIId == nil {
		return ""
	}
	return p.FIId.ISPB()
}

// NewAppHdr returns a header from one ISPB to another for a business
// message identified by bizMsgIdr carrying the message type msgDefIdr
// (e.g. "pacs.008.001.08").
func NewAppHdr(fromISPB, toISPB, bizMsgIdr, msgDefIdr string, created time.Time) *AppHdr {
	from, to := xsd.AgentISPB(fromISPB), xsd.AgentISPB(toISPB)
	return &AppHdr{
		XMLName:   xml.Name{Space: AppHdrNamespace, Local: "AppHdr"},
		Fr:        HeaderParty{FIId: &from},
		To:        HeaderParty{FIId: &to},
		BizMsgIdr: bizMsgIdr,
		MsgDefIdr: msgDefIdr,
		CreDt:     xsd.NewDateTime(created),
	}
}

// Validate checks the header and returns every violation as
// xsd.Violations, or nil: Fr and To identified by 8-digit ISPB,
// BizMsgIdr (Max35Text), MsgDefIdr a message identifier such as
// pacs.008.001.08, and CreDt an ISODateTime.
func (h *AppHdr) Validate() error {
	var c check.Checker
	const p = "/AppHdr"
	for _, side := range []struct {
		name string
		hp   HeaderParty
	}{{"Fr", h.Fr}, {"To", h.To}} {
		if side.hp.FIId == nil {
			c.Addf(p+"/"+side.name+"/FIId", "is required")
			continue
		}
		c.Agent(p+"/"+side.name+"/FIId", *side.hp.FIId)
	}
	c.Max35(p+"/BizMsgIdr", h.BizMsgIdr, true)
	if h.MsgDefIdr == "" {
		c.Addf(p+"/MsgDefIdr", "is required")
	} else if _, ok := xmlutil.ParseMessageID(h.MsgDefIdr); !ok {
		c.Addf(p+"/MsgDefIdr", "must be a message identifier such as pacs.008.001.08, got %q", h.MsgDefIdr)
	}
	c.Max35(p+"/BizSvc", h.BizSvc, false)
	c.DateTime(p+"/CreDt", h.CreDt, true)
	return c.Err()
}

// Marshal renders the header as indented XML with an XML declaration.
func (h *AppHdr) Marshal() ([]byte, error) {
	c := *h
	if c.XMLName.Space == "" {
		c.XMLName = xml.Name{Space: AppHdrNamespace, Local: "AppHdr"}
	}
	c.XMLName.Local = "AppHdr"
	return xmlutil.Marshal(&c)
}

// Envelope is the result of Inspect: the optional AppHdr and the type of
// the Document.
type Envelope struct {
	Header *AppHdr // nil when the payload is a bare Document
	Type   MessageType
}

// Validate checks the header, when present, with AppHdr.Validate and
// verifies that its MsgDefIdr matches the Document namespace.
func (e Envelope) Validate() error {
	if e.Header == nil {
		return nil
	}
	var vs xsd.Violations
	if err := e.Header.Validate(); err != nil {
		vs = append(vs, err.(xsd.Violations)...)
	}
	if e.Header.MsgDefIdr != "" && e.Header.MsgDefIdr != e.Type.String() {
		vs = append(vs, xsd.Violation{
			Path:    "/AppHdr/MsgDefIdr",
			Message: fmt.Sprintf("is %q but the Document is %s", e.Header.MsgDefIdr, e.Type),
		})
	}
	if len(vs) == 0 {
		return nil
	}
	return vs
}

// Inspect is Detect plus decoding of a head.001 AppHdr that precedes the
// Document. Like Detect, it stops at the Document start element.
func Inspect(data []byte) (Envelope, error) {
	return InspectReader(bytes.NewReader(data))
}

// InspectReader is Inspect for a stream.
func InspectReader(r io.Reader) (Envelope, error) {
	s := xmlutil.NewScanner(r)
	var env Envelope
	for {
		se, err := s.Next()
		if err == io.EOF {
			return Envelope{}, ErrNoDocument
		}
		if err != nil {
			return Envelope{}, err
		}
		if se.Name.Local == "AppHdr" {
			if env.Header != nil {
				return Envelope{}, fmt.Errorf("iso20022: more than one AppHdr")
			}
			var h AppHdr
			if err := s.D.DecodeElement(&h, &se); err != nil {
				return Envelope{}, err
			}
			env.Header = &h
			continue
		}
		if xmlutil.IsDocument(se) {
			id, err := xmlutil.DocumentID(se)
			if err != nil {
				return Envelope{}, err
			}
			env.Type = MessageType{id.BusinessArea, id.Function, id.Variant, id.Version}
			return env, nil
		}
	}
}
