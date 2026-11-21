package xmlutil

import (
	"encoding/xml"
	"testing"
)

func TestParseMessageID(t *testing.T) {
	id, ok := ParseMessageID(NamespacePrefix + "pacs.008.001.08")
	if !ok || id.String() != "pacs.008.001.08" || id.Base() != "pacs.008" || id.Namespace() != NamespacePrefix+"pacs.008.001.08" {
		t.Fatal(id, ok)
	}
	for _, bad := range []string{"", "pacs.008.001", "pac.008.001.08", "pacs.008.001.8", "pacs.008.0x1.08", "Pacs.008.001.08"} {
		if _, ok := ParseMessageID(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestMarshalError(t *testing.T) {
	if _, err := Marshal(make(chan int)); err == nil {
		t.Error("channel marshalled")
	}
	if n := Name(xml.Name{Space: "x"}, "y"); n.Space != "x" || n.Local != "Document" {
		t.Error(n)
	}
}

func TestDecodeDocumentErrors(t *testing.T) {
	var v struct{}
	if _, err := DecodeDocument([]byte(`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"><a></b></Document>`), "pacs.008", &v); err == nil {
		t.Error("mismatched tags accepted")
	}
}
