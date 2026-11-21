package pacs002

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/pacs008"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

var update = flag.Bool("update", false, "rewrite golden files")

func load(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func original(t testing.TB) *pacs008.Document {
	t.Helper()
	d, err := pacs008.Parse(load(t, "pacs008.xml"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGolden(t *testing.T) {
	for _, name := range []string{"pacs002_accept.xml", "pacs002_reject.xml"} {
		t.Run(name, func(t *testing.T) {
			data := load(t, name)
			d, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Validate(); err != nil {
				t.Fatal(err)
			}
			out, err := d.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				os.WriteFile("../testdata/"+name, out, 0o644)
			}
			if !bytes.Equal(out, data) {
				t.Errorf("Marshal(Parse(golden)) differs:\n%s", out)
			}
			if d.Version() != "10" {
				t.Errorf("version %q", d.Version())
			}
		})
	}
	d, _ := Parse(load(t, "pacs002_reject.xml"))
	tx := d.Transactions()[0]
	if tx.Accepted() || tx.Reason() != "AC03" {
		t.Errorf("reject: accepted=%v reason=%q", tx.Accepted(), tx.Reason())
	}
}

func TestAccept(t *testing.T) {
	at := time.Date(2026, 11, 21, 10, 30, 1, 456e6, time.UTC)
	d, err := Accept(original(t), At(at), SettledAt(at.Add(333*time.Millisecond)), WithMsgID("M8888888820261121103000000000007"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if want := load(t, "pacs002_accept.xml"); !bytes.Equal(out, want) {
		t.Errorf("Accept output differs from golden:\n%s", out)
	}
	if !d.Transactions()[0].Accepted() || d.Transactions()[0].Reason() != "" {
		t.Error("not accepted")
	}
}

func TestReject(t *testing.T) {
	at := time.Date(2026, 11, 21, 10, 30, 1, 456e6, time.UTC)
	d, err := Reject(original(t), "AC03", At(at), WithMsgID("M8888888820261121103000000000008"),
		WithAdditionalInfo("Conta do recebedor inexistente"))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := d.Marshal()
	if want := load(t, "pacs002_reject.xml"); !bytes.Equal(out, want) {
		t.Errorf("Reject output differs from golden:\n%s", out)
	}

	// Generated MsgId uses the creditor agent's ISPB by default.
	d, err = Reject(original(t), "AM04")
	if err != nil {
		t.Fatal(err)
	}
	if id := d.FIToFIPmtStsRpt.GrpHdr.MsgId; !strings.HasPrefix(id, "M88888888") || len(id) != 32 {
		t.Errorf("MsgId %q", id)
	}
	d, err = Accept(original(t), WithISPB("77777777"))
	if err != nil || !strings.HasPrefix(d.FIToFIPmtStsRpt.GrpHdr.MsgId, "M77777777") {
		t.Errorf("WithISPB: %v", err)
	}

	if _, err := Reject(original(t), "ZZ99"); !errors.Is(err, ErrUnknownReason) {
		t.Errorf("unknown reason: %v", err)
	}
	if _, err := Reject(original(t), "ZZ99", AllowUnknownReason(), WithRand(bytes.NewReader(make([]byte, 64)))); err != nil {
		t.Errorf("AllowUnknownReason: %v", err)
	}
	if _, err := Reject(original(t), ""); err == nil {
		t.Error("empty reason accepted")
	}
	if _, err := Reject(original(t), "TOOLONG", AllowUnknownReason()); err == nil {
		t.Error("5-char code accepted")
	}
	if _, err := Accept(&pacs008.Document{}); err == nil {
		t.Error("empty original accepted")
	}
	if _, err := Accept(nil); err == nil {
		t.Error("nil original accepted")
	}
	if _, err := Accept(original(t), WithISPB("x")); err == nil {
		t.Error("bad ISPB accepted")
	}
}

func TestValidate(t *testing.T) {
	base, _ := Parse(load(t, "pacs002_reject.xml"))
	tests := []struct {
		name   string
		mutate func(d *Document)
		want   string
	}{
		{"namespace", func(d *Document) { d.XMLName.Space = "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08" }, "must be a pacs.002 namespace"},
		{"msgid", func(d *Document) { d.FIToFIPmtStsRpt.GrpHdr.MsgId = "" }, "GrpHdr/MsgId: is required"},
		{"orgnl", func(d *Document) { d.FIToFIPmtStsRpt.OrgnlGrpInfAndSts[0].OrgnlMsgNmId = "" }, "OrgnlGrpInfAndSts[0]/OrgnlMsgNmId: is required"},
		{"no tx", func(d *Document) { d.FIToFIPmtStsRpt.TxInfAndSts = nil }, "at least one transaction status is required"},
		{"e2e", func(d *Document) { d.FIToFIPmtStsRpt.TxInfAndSts[0].OrgnlEndToEndId = "" }, "TxInfAndSts[0]/OrgnlEndToEndId: is required"},
		{"status", func(d *Document) { d.FIToFIPmtStsRpt.TxInfAndSts[0].TxSts = "" }, "TxInfAndSts[0]/TxSts: is required"},
		{"no reason", func(d *Document) { d.FIToFIPmtStsRpt.TxInfAndSts[0].StsRsnInf = nil }, "a rejection (RJCT) requires a reason code"},
		{"addtlinf", func(d *Document) {
			d.FIToFIPmtStsRpt.TxInfAndSts[0].StsRsnInf[0].AddtlInf = []string{strings.Repeat("x", 106)}
		}, "StsRsnInf[0]/AddtlInf[0]: must be at most 105 characters"},
		{"settlement both", func(d *Document) {
			d.FIToFIPmtStsRpt.TxInfAndSts[0].FctvIntrBkSttlmDt = &xsd.DateAndDateTime{Dt: "2026-11-21", DtTm: "2026-11-21T10:00:00Z"}
		}, "must contain exactly one of Dt or DtTm, got both"},
		{"settlement none", func(d *Document) { d.FIToFIPmtStsRpt.TxInfAndSts[0].FctvIntrBkSttlmDt = &xsd.DateAndDateTime{} }, "must contain exactly one of Dt or DtTm"},
		{"settlement date", func(d *Document) {
			d.FIToFIPmtStsRpt.TxInfAndSts[0].FctvIntrBkSttlmDt = &xsd.DateAndDateTime{Dt: "21-11-2026"}
		}, "FctvIntrBkSttlmDt/Dt: must be an ISO 8601 date"},
		{"settlement datetime", func(d *Document) {
			d.FIToFIPmtStsRpt.TxInfAndSts[0].FctvIntrBkSttlmDt = &xsd.DateAndDateTime{DtTm: "x"}
		}, "FctvIntrBkSttlmDt/DtTm: must be an ISO 8601 date and time"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := base.Marshal()
			d, _ := Parse(b)
			tt.mutate(d)
			if err := d.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
	// Unknown reason codes are valid and preserved.
	b, _ := base.Marshal()
	b = bytes.Replace(b, []byte("AC03"), []byte("XY12"), 1)
	d, err := Parse(b)
	if err != nil || d.Validate() != nil || d.Transactions()[0].Reason() != "XY12" {
		t.Errorf("unknown code: %v", err)
	}
	if _, err := Parse(load(t, "pacs008.xml")); !errors.Is(err, xmlutil.ErrUnexpectedMessage) {
		t.Errorf("pacs.008 parsed as pacs.002: %v", err)
	}
	if (&Document{}).Version() != "" {
		t.Error("Version of empty document")
	}
}

func TestReasons(t *testing.T) {
	info, ok := Reasons.Lookup("AM04")
	if !ok || info.Name != "InsufficientFunds" || info.Portuguese == "" || info.English == "" {
		t.Fatalf("AM04: %+v", info)
	}
	if _, ok := Reasons.Lookup("ZZZZ"); ok {
		t.Error("ZZZZ known")
	}
	all := Reasons.Sorted()
	for i, c := range all {
		if len(c.Code) != 4 || c.Name == "" || c.English == "" || c.Portuguese == "" {
			t.Errorf("incomplete entry %+v", c)
		}
		if i > 0 && all[i-1].Code >= c.Code {
			t.Error("not sorted")
		}
	}
}

func FuzzRoundTrip(f *testing.F) {
	f.Add(load(f, "pacs002_accept.xml"))
	f.Add(load(f, "pacs002_reject.xml"))
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Parse(data)
		if err != nil {
			return
		}
		_ = d.Validate()
		out, err := d.Marshal()
		if err != nil {
			return
		}
		d2, err := Parse(out)
		if err != nil {
			t.Fatalf("re-parse: %v\n%s", err, out)
		}
		if !reflect.DeepEqual(d, d2) {
			t.Fatalf("mismatch:\n%#v\n%#v", d, d2)
		}
	})
}
