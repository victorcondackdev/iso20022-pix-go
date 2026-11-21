// Package xmlutil contains the XML plumbing shared by the root package and
// the message packages: namespace parsing, locating the Document element
// with a streaming decoder, strict decoding and canonical marshalling.
package xmlutil

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// NamespacePrefix is the common prefix of ISO 20022 XML namespaces.
const NamespacePrefix = "urn:iso:std:iso:20022:tech:xsd:"

// Errors shared with the root package, which re-exports them.
var (
	ErrNoDocument         = errors.New("iso20022: no ISO 20022 Document element found")
	ErrUnknownNamespace   = errors.New("iso20022: Document namespace is not an ISO 20022 message namespace")
	ErrUnexpectedMessage  = errors.New("iso20022: unexpected message type")
	ErrDirectiveForbidden = errors.New("iso20022: XML directives (DOCTYPE, ENTITY) are not allowed")
	ErrMultipleDocuments  = errors.New("iso20022: more than one Document element")
)

// MessageID is a parsed message identifier such as pacs.008.001.08.
type MessageID struct {
	BusinessArea string // "pacs"
	Function     string // "008"
	Variant      string // "001"
	Version      string // "08"
}

// ParseMessageID parses "pacs.008.001.08" or the full namespace
// "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08".
func ParseMessageID(s string) (MessageID, bool) {
	s = strings.TrimPrefix(s, NamespacePrefix)
	parts := strings.Split(s, ".")
	if len(parts) != 4 || len(parts[0]) != 4 || len(parts[1]) != 3 || len(parts[2]) != 3 || len(parts[3]) != 2 {
		return MessageID{}, false
	}
	for _, c := range []byte(parts[0]) {
		if c < 'a' || c > 'z' {
			return MessageID{}, false
		}
	}
	for _, p := range parts[1:] {
		for _, c := range []byte(p) {
			if c < '0' || c > '9' {
				return MessageID{}, false
			}
		}
	}
	return MessageID{parts[0], parts[1], parts[2], parts[3]}, true
}

// String returns "pacs.008.001.08".
func (m MessageID) String() string {
	return m.BusinessArea + "." + m.Function + "." + m.Variant + "." + m.Version
}

// Base returns "pacs.008".
func (m MessageID) Base() string { return m.BusinessArea + "." + m.Function }

// Namespace returns the full XML namespace.
func (m MessageID) Namespace() string { return NamespacePrefix + m.String() }

// Scanner walks an XML stream token by token, rejecting directives.
type Scanner struct {
	D *xml.Decoder
}

// NewScanner returns a strict decoder over r.
func NewScanner(r io.Reader) *Scanner {
	d := xml.NewDecoder(r)
	d.Strict = true
	return &Scanner{D: d}
}

// Next returns the next start element, or io.EOF.
func (s *Scanner) Next() (xml.StartElement, error) {
	for {
		tok, err := s.D.Token()
		if err != nil {
			return xml.StartElement{}, err
		}
		switch t := tok.(type) {
		case xml.Directive:
			return xml.StartElement{}, ErrDirectiveForbidden
		case xml.StartElement:
			return t, nil
		}
	}
}

// IsDocument reports whether se is an ISO 20022 Document element.
func IsDocument(se xml.StartElement) bool { return se.Name.Local == "Document" }

// DocumentID returns the message identifier of a Document start element.
func DocumentID(se xml.StartElement) (MessageID, error) {
	id, ok := ParseMessageID(se.Name.Space)
	if !ok || !strings.HasPrefix(se.Name.Space, NamespacePrefix) {
		return MessageID{}, fmt.Errorf("%w: %q", ErrUnknownNamespace, se.Name.Space)
	}
	return id, nil
}

// DecodeDocument finds the Document element in data, which may be a bare
// Document or a business message wrapping an AppHdr and a Document,
// checks that its namespace is the message base (e.g. "pacs.008") in any
// version, decodes it into v and verifies that the rest of the input is
// well-formed and contains no second Document.
func DecodeDocument(data []byte, base string, v any) (MessageID, error) {
	s := NewScanner(bytes.NewReader(data))
	var (
		id    MessageID
		found bool
	)
	for {
		se, err := s.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return MessageID{}, err
		}
		if !IsDocument(se) {
			continue
		}
		if found {
			return MessageID{}, ErrMultipleDocuments
		}
		id, err = DocumentID(se)
		if err != nil {
			return MessageID{}, err
		}
		if id.Base() != base {
			return MessageID{}, fmt.Errorf("%w: got %s, want %s", ErrUnexpectedMessage, id, base)
		}
		if err := s.D.DecodeElement(v, &se); err != nil {
			return MessageID{}, err
		}
		found = true
	}
	if !found {
		return MessageID{}, ErrNoDocument
	}
	return id, nil
}

// Marshal renders v as an indented XML document with an XML declaration
// and a trailing newline.
func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	enc := xml.NewEncoder(&b)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// Name returns the Document XML name for the namespace of id, or of def
// when cur is empty.
func Name(cur xml.Name, def string) xml.Name {
	if cur.Space == "" {
		return xml.Name{Space: def, Local: "Document"}
	}
	return xml.Name{Space: cur.Space, Local: "Document"}
}

// DocName returns the Document element name in namespace ns.
func DocName(ns string) xml.Name { return xml.Name{Space: ns, Local: "Document"} }
