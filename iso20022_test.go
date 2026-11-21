package iso20022

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

func load(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDetectGolden(t *testing.T) {
	tests := map[string]string{
		"pacs008.xml":          "pacs.008.001.08",
		"envelope_pacs008.xml": "pacs.008.001.08",
		"pacs002_accept.xml":   "pacs.002.001.10",
		"pacs002_reject.xml":   "pacs.002.001.10",
		"pacs004.xml":          "pacs.004.001.09",
		"camt054.xml":          "camt.054.001.08",
	}
	for file, want := range tests {
		mt, err := Detect(load(t, file))
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if mt.String() != want || !mt.Supported() || mt.Name() == "" {
			t.Errorf("%s: got %s (%q)", file, mt, mt.Name())
		}
		if mt.Namespace() != "urn:iso:std:iso:20022:tech:xsd:"+want {
			t.Errorf("namespace %s", mt.Namespace())
		}
	}
}

func TestDetectVersions(t *testing.T) {
	for _, in := range []string{
		`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.13"/>`,
		`<ns:Document xmlns:ns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.02"><ns:FIToFICstmrCdtTrf/></ns:Document>`,
	} {
		mt, err := Detect([]byte(in))
		if err != nil || mt.ID() != Pacs008 {
			t.Errorf("%s: %v %v", in, mt, err)
		}
	}
	mt, _ := Detect([]byte(`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.08"/>`))
	if mt.Supported() || mt.Name() != "" || mt.Version != "08" {
		t.Errorf("camt.053: %+v", mt)
	}
}

func TestDetectErrors(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{``, ErrNoDocument},
		{`<a><b/></a>`, ErrNoDocument},
		{`<Document/>`, ErrUnknownNamespace},
		{`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008"/>`, ErrUnknownNamespace},
		{`<Document xmlns="urn:x:pacs.008.001.08"/>`, ErrUnknownNamespace},
		{`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:PACS.008.001.08"/>`, ErrUnknownNamespace},
		{`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.0a8.001.08"/>`, ErrUnknownNamespace},
		{`<!DOCTYPE Document><Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"/>`, ErrDirectiveForbidden},
	}
	for _, tt := range tests {
		if _, err := Detect([]byte(tt.in)); !errors.Is(err, tt.want) {
			t.Errorf("Detect(%q) = %v, want %v", tt.in, err, tt.want)
		}
	}
	if _, err := Detect([]byte(`<a><b></a>`)); err == nil {
		t.Error("malformed accepted")
	}
	if _, err := Detect([]byte(`<E><AppHdr><x></AppHdr></E>`)); err == nil {
		t.Error("malformed AppHdr accepted")
	}
}

// limitedReader counts the bytes read and returns at most 64 per call.
type limitedReader struct {
	r    io.Reader
	read int
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if len(p) > 64 {
		p = p[:64]
	}
	n, err := l.r.Read(p)
	l.read += n
	return n, err
}

func TestDetectIsStreaming(t *testing.T) {
	// A huge body after the Document start tag is never read.
	head := `<?xml version="1.0"?><Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08">`
	body := strings.Repeat("<x>filler</x>", 1<<16)
	lr := &limitedReader{r: strings.NewReader(head + body + "</Document>")}
	mt, err := DetectReader(lr)
	if err != nil || mt.ID() != Pacs008 {
		t.Fatal(mt, err)
	}
	if lr.read > len(head)+4096 {
		t.Errorf("read %d bytes for a %d-byte header", lr.read, len(head))
	}
}

func TestParseMessageType(t *testing.T) {
	mt, err := ParseMessageType("pacs.004.001.09")
	if err != nil || mt.ID() != Pacs004 || mt.Name() != "PaymentReturn" {
		t.Fatal(mt, err)
	}
	if _, err := ParseMessageType("pacs.4.1.9"); err == nil {
		t.Error("accepted short parts")
	}
	if (MessageType{}).String() != "" || (MessageType{}).ID() != "" {
		t.Error("zero MessageType")
	}
	if !strings.Contains(MessageType{"head", "001", "001", "01"}.Name(), "Header") {
		t.Error("head.001 name")
	}
}

func TestInspect(t *testing.T) {
	env, err := Inspect(load(t, "envelope_pacs008.xml"))
	if err != nil {
		t.Fatal(err)
	}
	h := env.Header
	if h == nil || h.Fr.ISPB() != "99999999" || h.To.ISPB() != "00038166" || h.MsgDefIdr != "pacs.008.001.08" {
		t.Fatalf("header %+v", h)
	}
	if err := env.Validate(); err != nil {
		t.Fatal(err)
	}
	if env.Type.ID() != Pacs008 {
		t.Error(env.Type)
	}

	bare, err := Inspect(load(t, "pacs008.xml"))
	if err != nil || bare.Header != nil || bare.Validate() != nil {
		t.Fatalf("bare: %+v %v", bare, err)
	}

	mismatch := bytes.Replace(load(t, "envelope_pacs008.xml"), []byte("<MsgDefIdr>pacs.008.001.08"), []byte("<MsgDefIdr>pacs.002.001.10"), 1)
	env, err = Inspect(mismatch)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.Validate(); err == nil || !strings.Contains(err.Error(), `/AppHdr/MsgDefIdr: is "pacs.002.001.10" but the Document is pacs.008.001.08`) {
		t.Errorf("mismatch: %v", err)
	}

	for _, bad := range []string{
		`<E><AppHdr/><AppHdr/><Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"/></E>`,
		`<E><AppHdr><Fr></AppHdr></E>`,
		`<E><AppHdr/></E>`,
		`<E><AppHdr/><Document/></E>`,
		`<E`,
	} {
		if _, err := Inspect([]byte(bad)); err == nil {
			t.Errorf("Inspect(%q) accepted", bad)
		}
	}
}

func TestAppHdr(t *testing.T) {
	at := time.Date(2026, 11, 21, 10, 30, 1, 123e6, time.UTC)
	h := NewAppHdr("99999999", "00038166", "M9999999920261121103000000000001", "pacs.008.001.08", at)
	if err := h.Validate(); err != nil {
		t.Fatal(err)
	}
	b, err := h.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	env := append(append([]byte("<E>"), bytes.TrimPrefix(b, []byte(`<?xml version="1.0" encoding="UTF-8"?>`))...),
		[]byte(`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"/></E>`)...)
	got, err := Inspect(env)
	if err != nil {
		t.Fatal(err)
	}
	if got.Header.BizMsgIdr != h.BizMsgIdr || got.Header.CreDt != "2026-11-21T10:30:01.123Z" || got.Validate() != nil {
		t.Errorf("round trip %+v", got.Header)
	}
	if !bytes.Contains(b, []byte(`<AppHdr xmlns="`+AppHdrNamespace+`">`)) {
		t.Errorf("%s", b)
	}
	b, _ = (&AppHdr{}).Marshal()
	if !bytes.Contains(b, []byte(`<AppHdr xmlns="`+AppHdrNamespace+`">`)) {
		t.Errorf("zero header: %s", b)
	}

	bad := &AppHdr{To: HeaderParty{FIId: &xsd.Agent{}}, MsgDefIdr: "pacs8", BizSvc: strings.Repeat("s", 36), CreDt: "yesterday"}
	err = bad.Validate()
	var vs xsd.Violations
	if !errors.As(err, &vs) || len(vs) != 6 {
		t.Fatalf("got %v", err)
	}
	for i, frag := range []string{"/AppHdr/Fr/FIId: is required", "/AppHdr/To/FIId/FinInstnId/ClrSysMmbId/MmbId", "/AppHdr/BizMsgIdr: is required",
		"/AppHdr/MsgDefIdr: must be a message identifier", "/AppHdr/BizSvc", "/AppHdr/CreDt"} {
		if !strings.HasPrefix(vs[i].Error(), frag) {
			t.Errorf("violation %d = %s, want prefix %s", i, vs[i], frag)
		}
	}
	if err := (&AppHdr{}).Validate(); err == nil || !strings.Contains(err.Error(), "/AppHdr/MsgDefIdr: is required") {
		t.Errorf("empty: %v", err)
	}
	if (HeaderParty{}).ISPB() != "" {
		t.Error("empty party ISPB")
	}
}

func FuzzDetect(f *testing.F) {
	for _, n := range []string{"pacs008.xml", "envelope_pacs008.xml", "pacs004.xml"} {
		f.Add(load(f, n))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		mt, err := Detect(data)
		env, err2 := Inspect(data)
		if err == nil {
			if mt.String() == "" {
				t.Fatal("empty type without error")
			}
			if _, perr := ParseMessageType(mt.Namespace()); perr != nil {
				t.Fatalf("detected type %s does not re-parse", mt)
			}
		}
		if err2 == nil {
			_ = env.Validate()
			if err == nil && env.Type != mt {
				t.Fatalf("Detect %s vs Inspect %s", mt, env.Type)
			}
		}
	})
}
