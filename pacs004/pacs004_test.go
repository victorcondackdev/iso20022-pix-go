package pacs004

import (
	"bytes"
	"errors"
	"flag"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/ids"
	"github.com/victorcondackdev/iso20022-pix-go/money"
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

var at = time.Date(2026, 11, 21, 15, 0, 0, 0, time.UTC)

func TestGolden(t *testing.T) {
	data := load(t, "pacs004.xml")
	d, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	out, _ := d.Marshal()
	if *update {
		os.WriteFile("../testdata/pacs004.xml", out, 0o644)
	}
	if !bytes.Equal(out, data) {
		t.Errorf("Marshal(Parse(golden)) differs:\n%s", out)
	}
	tx := d.Transactions()[0]
	if tx.Amount() != money.MustParse("50") || tx.Reason() != ReasonCustomerRefund || d.Version() != "09" {
		t.Errorf("unexpected %+v", tx)
	}
}

func TestFromPacs008(t *testing.T) {
	d, err := FromPacs008(original(t), ReasonCustomerRefund, money.MustParse("50.00"),
		At(at),
		WithMsgID("M8888888820261121150000000000003"),
		WithReturnID("D88888888202611211500Zx9Yw8Vu7Ts"),
		WithAdditionalInfo("Devolucao parcial a pedido do recebedor"))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := d.Marshal()
	if want := load(t, "pacs004.xml"); !bytes.Equal(out, want) {
		t.Errorf("FromPacs008 output differs from golden:\n%s", out)
	}

	// Generated identifiers belong to the creditor's participant.
	d, err = FromPacs008(original(t), ReasonFraud, money.MustParse("150.75"), At(at))
	if err != nil {
		t.Fatal(err)
	}
	tx := d.Transactions()[0]
	rid, err := ids.ParseReturnID(tx.RtrId)
	if err != nil || rid.ISPB != "88888888" || !rid.Time.Equal(at) {
		t.Errorf("RtrId %q: %v", tx.RtrId, err)
	}
	if !strings.HasPrefix(d.PmtRtr.GrpHdr.MsgId, "M88888888") {
		t.Errorf("MsgId %q", d.PmtRtr.GrpHdr.MsgId)
	}
}

func TestFromPacs008Limits(t *testing.T) {
	orig := original(t) // 150.75
	tests := []struct {
		name   string
		amount string
		opts   []Option
		want   error
	}{
		{"full", "150.75", nil, nil},
		{"partial", "0.01", nil, nil},
		{"over", "150.76", nil, ErrAmountExceeded},
		{"partial after prior", "100.75", []Option{WithPriorReturns(money.MustParse("50"))}, nil},
		{"over after prior", "100.76", []Option{WithPriorReturns(money.MustParse("50"))}, ErrAmountExceeded},
		{"prior above original", "0.01", []Option{WithPriorReturns(money.MustParse("200"))}, ErrAmountExceeded},
		{"negative prior", "0.01", []Option{WithPriorReturns(money.Amount(math.MinInt64 + 1))}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromPacs008(orig, ReasonBankError, money.MustParse(tt.amount), tt.opts...)
			if tt.name == "negative prior" {
				if err == nil {
					t.Fatal("negative prior accepted")
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
	if _, err := FromPacs008(orig, ReasonBankError, 0); err == nil {
		t.Error("zero amount accepted")
	}
	if _, err := FromPacs008(orig, "XX01", 1); !errors.Is(err, ErrUnknownReason) {
		t.Errorf("unknown reason: %v", err)
	}
	if _, err := FromPacs008(orig, "XX01", 1, AllowUnknownReason()); err != nil {
		t.Errorf("AllowUnknownReason: %v", err)
	}
	if _, err := FromPacs008(nil, ReasonBankError, 1); !errors.Is(err, ErrNoTransaction) {
		t.Errorf("nil original: %v", err)
	}
	if _, err := FromPacs008(&pacs008.Document{}, ReasonBankError, 1); !errors.Is(err, ErrNoTransaction) {
		t.Errorf("empty original: %v", err)
	}
	if _, err := FromPacs008(orig, ReasonBankError, 1, ForEndToEndID("E99999999202611211030a1B2c3D4e5F")); err != nil {
		t.Errorf("ForEndToEndID: %v", err)
	}
	if _, err := FromPacs008(orig, ReasonBankError, 1, ForEndToEndID("nope")); !errors.Is(err, ErrNoTransaction) {
		t.Errorf("ForEndToEndID missing: %v", err)
	}
	bad := original(t)
	bad.FIToFICstmrCdtTrf.CdtTrfTxInf[0].CdtrAgt = xsd.AgentISPB("x")
	if _, err := FromPacs008(bad, ReasonBankError, 1); err == nil {
		t.Error("invalid creditor ISPB accepted for MsgId")
	}
	if _, err := FromPacs008(bad, ReasonBankError, 1, WithMsgID("m")); err == nil {
		t.Error("invalid creditor ISPB accepted for RtrId")
	}
	d, err := FromPacs008(orig, ReasonBankError, 1, WithRand(bytes.NewReader(bytes.Repeat([]byte{1}, 128))))
	if err != nil || !strings.HasSuffix(d.Transactions()[0].RtrId, "11111111111") {
		t.Errorf("WithRand: %v", err)
	}
	if _, err := FromPacs008(orig, ReasonBankError, 1, WithAdditionalInfo(strings.Repeat("x", 106))); err == nil {
		t.Error("long AddtlInf accepted")
	}
}

func TestValidate(t *testing.T) {
	base, _ := Parse(load(t, "pacs004.xml"))
	orig := xsd.BRL(money.MustParse("40"))
	tests := []struct {
		name   string
		mutate func(d *Document, tx *ReturnTransaction)
		want   string
	}{
		{"namespace", func(d *Document, _ *ReturnTransaction) { d.XMLName.Space = "urn:x" }, "must be a pacs.004 namespace"},
		{"rtrid", func(_ *Document, tx *ReturnTransaction) { tx.RtrId = "E99999999202611211030a1B2c3D4e5F" }, `RtrId: must start with 'D'`},
		{"no rtrid", func(_ *Document, tx *ReturnTransaction) { tx.RtrId = "" }, "RtrId: is required"},
		{"over original", func(_ *Document, tx *ReturnTransaction) { tx.OrgnlIntrBkSttlmAmt = &orig }, "RtrdIntrBkSttlmAmt: 50.00 exceeds OrgnlIntrBkSttlmAmt 40.00"},
		{"zero", func(_ *Document, tx *ReturnTransaction) { tx.RtrdIntrBkSttlmAmt.Value = 0 }, "RtrdIntrBkSttlmAmt: must be greater than zero"},
		{"chrgbr", func(_ *Document, tx *ReturnTransaction) { tx.ChrgBr = "DEBT" }, "ChrgBr: must be one of [SLEV]"},
		{"no reason", func(_ *Document, tx *ReturnTransaction) { tx.RtrRsnInf = nil }, "RtrRsnInf: a return reason code is required"},
		{"orgnl msg", func(_ *Document, tx *ReturnTransaction) { tx.OrgnlGrpInf.OrgnlMsgId = "" }, "OrgnlGrpInf/OrgnlMsgId: is required"},
		{"nboftxs", func(d *Document, _ *ReturnTransaction) { d.PmtRtr.GrpHdr.NbOfTxs = "3" }, "NbOfTxs: is 3"},
		{"no tx", func(d *Document, _ *ReturnTransaction) { d.PmtRtr.TxInf = d.PmtRtr.TxInf[:0] }, "TxInf: at least one transaction is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := base.Marshal()
			d, _ := Parse(b)
			tt.mutate(d, &d.PmtRtr.TxInf[0])
			if err := d.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
	if (&Document{}).Version() != "" || (&ReturnTransaction{}).Reason() != "" {
		t.Error("zero values")
	}
	if _, err := Parse([]byte("<x/>")); err == nil {
		t.Error("parsed non-message")
	}
}

func TestReasons(t *testing.T) {
	for _, c := range []string{ReasonBankError, ReasonFraud, ReasonCustomerRefund, ReasonCreditorService} {
		if info, ok := Reasons.Lookup(c); !ok || info.Portuguese == "" {
			t.Errorf("%s missing", c)
		}
	}
}

func FuzzRoundTrip(f *testing.F) {
	f.Add(load(f, "pacs004.xml"))
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

// FuzzFromPacs008 checks the partial-return invariant: a return is built
// if and only if 0 < amount <= original - prior.
func FuzzFromPacs008(f *testing.F) {
	f.Add(int64(5000), int64(0))
	f.Add(int64(15075), int64(1))
	f.Add(int64(1), int64(15075))
	orig := original(f)
	paid := orig.Transactions()[0].Amount()
	f.Fuzz(func(t *testing.T, amount, prior int64) {
		d, err := FromPacs008(orig, ReasonBankError, money.Amount(amount), WithPriorReturns(money.Amount(prior)), At(at))
		ok := amount > 0 && prior >= 0 && prior <= int64(paid) && amount <= int64(paid)-prior
		if ok != (err == nil) {
			t.Fatalf("amount=%d prior=%d: err=%v, want ok=%v", amount, prior, err, ok)
		}
		if err == nil && d.Transactions()[0].Amount() != money.Amount(amount) {
			t.Fatal("amount not carried")
		}
	})
}
