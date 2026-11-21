package camt054

import (
	"bytes"
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

var update = flag.Bool("update", false, "rewrite golden files")

const golden = "../testdata/camt054.xml"

func load(t testing.TB) []byte {
	t.Helper()
	b, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGolden(t *testing.T) {
	data := load(t)
	d, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	out, _ := d.Marshal()
	if *update {
		os.WriteFile(golden, out, 0o644)
	}
	if !bytes.Equal(out, data) {
		t.Errorf("Marshal(Parse(golden)) differs:\n%s", out)
	}
	es := d.Entries()
	if len(es) != 2 || d.Version() != "08" {
		t.Fatalf("%d entries, version %q", len(es), d.Version())
	}
	var net money.Amount
	for i := range es {
		net += es[i].Signed()
	}
	if net != money.MustParse("100.75") {
		t.Errorf("net %s", net)
	}
	if got := es[0].EndToEndIDs(); len(got) != 1 || got[0] != "E99999999202611211030a1B2c3D4e5F" {
		t.Errorf("EndToEndIDs %v", got)
	}
}

func TestValidate(t *testing.T) {
	base, _ := Parse(load(t))
	yes := true
	tests := []struct {
		name   string
		mutate func(d *Document, e *Entry)
		want   string
	}{
		{"namespace", func(d *Document, _ *Entry) { d.XMLName.Space = "urn:iso:std:iso:20022:tech:xsd:camt.053.001.08" }, "must be a camt.054 namespace"},
		{"msgid", func(d *Document, _ *Entry) { d.BkToCstmrDbtCdtNtfctn.GrpHdr.MsgId = "" }, "GrpHdr/MsgId: is required"},
		{"no ntfctn", func(d *Document, _ *Entry) { d.BkToCstmrDbtCdtNtfctn.Ntfctn = nil }, "Ntfctn: at least one notification is required"},
		{"no acct", func(d *Document, _ *Entry) { d.BkToCstmrDbtCdtNtfctn.Ntfctn[0].Acct = xsd.Account{} }, "Ntfctn[0]/Acct/Id: is required"},
		{"iban", func(d *Document, _ *Entry) {
			d.BkToCstmrDbtCdtNtfctn.Ntfctn[0].Acct = xsd.Account{Id: xsd.AccountID{IBAN: strings.Repeat("B", 35)}}
		}, "Acct/Id/IBAN: must be at most 34"},
		{"indicator", func(_ *Document, e *Entry) { e.CdtDbtInd = "CR" }, `Ntry[0]/CdtDbtInd: must be one of [CRDT DBIT], got "CR"`},
		{"status", func(_ *Document, e *Entry) { e.Sts = xsd.CodeOrProprietary{} }, "Ntry[0]/Sts: is required"},
		{"status both", func(_ *Document, e *Entry) { e.Sts.Prtry = "X" }, "Ntry[0]/Sts: must contain exactly one of Cd or Prtry, got both"},
		{"booking date", func(_ *Document, e *Entry) { e.BookgDt = nil }, "Ntry[0]/BookgDt: is required for booked entries"},
		{"bktxcd", func(_ *Document, e *Entry) { e.BkTxCd = BankTransactionCode{} }, "Ntry[0]/BkTxCd: must contain Domn or Prtry"},
		{"bktxcd prtry", func(_ *Document, e *Entry) { e.BkTxCd.Prtry = &ProprietaryBankTxCode{} }, "BkTxCd/Prtry/Cd: is required"},
		{"bktxcd family", func(_ *Document, e *Entry) { e.BkTxCd.Domn.Fmly.SubFmlyCd = "" }, "BkTxCd/Domn/Fmly/SubFmlyCd: is required"},
		{"e2e", func(_ *Document, e *Entry) { e.NtryDtls[0].TxDtls[0].Refs.EndToEndId = "X" }, "NtryDtls[0]/TxDtls[0]/Refs/EndToEndId: must be 32 characters"},
		{"sum", func(_ *Document, e *Entry) { e.NtryDtls[0].TxDtls[0].Amt.Value = 1 }, "Ntry[0]/Amt: is 150.75 but the transaction details add up to 0.01"},
		{"currency", func(_ *Document, e *Entry) { e.Amt.Ccy = "BRZ" }, `Ntry[0]/Amt/@Ccy: must be "BRL"`},
		{"tx indicator", func(_ *Document, e *Entry) { e.NtryDtls[0].TxDtls[0].CdtDbtInd = "C" }, "TxDtls[0]/CdtDbtInd: must be one of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := base.Marshal()
			d, _ := Parse(b)
			tt.mutate(d, &d.BkToCstmrDbtCdtNtfctn.Ntfctn[0].Ntry[0])
			if err := d.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
	// A pending entry without booking date and with a proprietary code
	// is fine; so are details without amounts.
	b, _ := base.Marshal()
	d, _ := Parse(b)
	e := &d.BkToCstmrDbtCdtNtfctn.Ntfctn[0].Ntry[0]
	e.Sts = xsd.CodeOrProprietary{Cd: StatusPending}
	e.BookgDt = nil
	e.RvslInd = &yes
	e.BkTxCd = BankTransactionCode{Prtry: &ProprietaryBankTxCode{Cd: "PIX", Issr: "X"}}
	e.NtryDtls[0].TxDtls[0].Amt = nil
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if (&Document{}).Version() != "" {
		t.Error("Version")
	}
}

func FuzzRoundTrip(f *testing.F) {
	f.Add(load(f))
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
