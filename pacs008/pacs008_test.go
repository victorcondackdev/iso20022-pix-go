package pacs008

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

var update = flag.Bool("update", false, "rewrite golden files")

const golden = "../testdata/pacs008.xml"

func load(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGolden(t *testing.T) {
	data := load(t, golden)
	d, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("golden file does not validate: %v", err)
	}
	tx := d.Transactions()[0]
	if tx.PmtId.EndToEndId != "E99999999202611211030a1B2c3D4e5F" || tx.Amount() != money.MustParse("150.75") {
		t.Fatalf("unexpected transaction %+v", tx.PmtId)
	}
	if tx.Dbtr.TaxID() != "52998224725" || tx.Cdtr.TaxID() != "11222333000181" {
		t.Errorf("tax ids: %q %q", tx.Dbtr.TaxID(), tx.Cdtr.TaxID())
	}
	if tx.DbtrAgt.ISPB() != "99999999" || tx.CdtrAgt.ISPB() != "88888888" {
		t.Errorf("agents")
	}
	if tx.CdtrAcct.PixKey() != "pagamentos@example.com" || tx.DbtrAcct.Branch() != "0001" || tx.DbtrAcct.Number() != "123456" {
		t.Errorf("accounts")
	}
	if tx.Initiation() != InitiationDICT || tx.RemittanceInfo() != "Pedido 4521" {
		t.Errorf("initiation/remittance: %q %q", tx.Initiation(), tx.RemittanceInfo())
	}
	if d.Version() != "08" || d.MessageName() != "pacs.008.001.08" {
		t.Errorf("version %q name %q", d.Version(), d.MessageName())
	}
	out, err := d.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(golden, out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(out, data) {
		t.Errorf("Marshal(Parse(golden)) differs from golden:\n%s", out)
	}
}

func TestParseEnvelope(t *testing.T) {
	d, err := Parse(load(t, "../testdata/envelope_pacs008.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if d.Transactions()[0].Initiation() != "" || d.Transactions()[0].RemittanceInfo() != "" {
		t.Error("optional fields should be empty")
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"no document", `<Foo/>`, xmlutil.ErrNoDocument},
		{"other message", `<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.002.001.10"/>`, xmlutil.ErrUnexpectedMessage},
		{"bad namespace", `<Document xmlns="urn:example"/>`, xmlutil.ErrUnknownNamespace},
		{"doctype", `<!DOCTYPE x [<!ENTITY a "b">]><Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"/>`, xmlutil.ErrDirectiveForbidden},
		{"two documents", `<E><Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"/><Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"/></E>`, xmlutil.ErrMultipleDocuments},
		{"bad amount", `<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"><FIToFICstmrCdtTrf><CdtTrfTxInf><IntrBkSttlmAmt Ccy="BRL">1.234</IntrBkSttlmAmt></CdtTrfTxInf></FIToFICstmrCdtTrf></Document>`, money.ErrInvalidAmount},
	}
	for _, tt := range tests {
		if _, err := Parse([]byte(tt.in)); !errors.Is(err, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, err, tt.want)
		}
	}
	if _, err := Parse([]byte(`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08">`)); err == nil {
		t.Error("truncated document accepted")
	}
	if _, err := Parse([]byte(`<E><Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"/><x></E>`)); err == nil {
		t.Error("malformed trailer accepted")
	}
}

func TestParseOtherVersion(t *testing.T) {
	data := bytes.Replace(load(t, golden), []byte("pacs.008.001.08"), []byte("pacs.008.001.10"), 1)
	d, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if d.Version() != "10" {
		t.Fatalf("version %q", d.Version())
	}
}

func sampleTransfer() Transfer {
	return Transfer{
		Amount:          money.MustParse("10.00"),
		AcceptedAt:      time.Date(2026, 11, 21, 10, 30, 0, 0, time.UTC),
		Initiation:      InitiationManual,
		Debtor:          Party{Name: "Fulano de Tal", TaxID: "52998224725"},
		DebtorAccount:   Account{ISPB: "99999999", Branch: "0001", Number: "123456", Type: xsd.AccountCurrent},
		Creditor:        Party{Name: "Loja Exemplo Ltda", TaxID: "11222333000181"},
		CreditorAccount: Account{ISPB: "88888888", Number: "000123", Type: xsd.AccountPayments},
		PixKey:          "+5521987654321",
		Purpose:         "IPAY",
		RemittanceInfo:  "obrigado",
	}
}

func TestBuilder(t *testing.T) {
	created := time.Date(2026, 11, 21, 10, 30, 1, 0, time.UTC)
	d, err := NewBuilder(created).Add(sampleTransfer()).Build()
	if err != nil {
		t.Fatal(err)
	}
	h := d.FIToFICstmrCdtTrf.GrpHdr
	if len(h.MsgId) != 32 || h.NbOfTxs != "1" || h.CreDtTm != "2026-11-21T10:30:01.000Z" {
		t.Errorf("header %+v", h)
	}
	tx := d.Transactions()[0]
	if !strings.HasPrefix(tx.PmtId.EndToEndId, "E99999999202611211030") {
		t.Errorf("generated E2E %q", tx.PmtId.EndToEndId)
	}
	if tx.Purp.Value() != "IPAY" || tx.CdtrAcct.PixKey() != "+5521987654321" || tx.Dbtr.Id.PrvtId == nil || tx.Cdtr.Id.OrgId == nil {
		t.Errorf("mapping: %+v", tx)
	}
	// Explicit ids are kept.
	tr := sampleTransfer()
	tr.EndToEndID = "E99999999202611211030a1B2c3D4e5F"
	tr.AcceptedAt = time.Time{}
	tr.Priority = "HIGH"
	tr.Initiation = ""
	d, err = NewBuilder(created).MsgID("custom").Add(tr).Build()
	if err != nil {
		t.Fatal(err)
	}
	if d.FIToFICstmrCdtTrf.GrpHdr.MsgId != "custom" || d.Transactions()[0].AccptncDtTm != xsd.NewDateTime(created) || d.Transactions()[0].PmtTpInf.LclInstrm != nil {
		t.Errorf("explicit values not kept")
	}
	if _, err := NewBuilder(created).Build(); err == nil {
		t.Error("empty builder accepted")
	}
	// A deterministic random source yields deterministic identifiers.
	zero := func() *bytes.Reader { return bytes.NewReader(make([]byte, 256)) }
	d1, _ := NewBuilder(created).Rand(zero()).Add(sampleTransfer()).Build()
	d2, _ := NewBuilder(created).Rand(zero()).Add(sampleTransfer()).Build()
	if !reflect.DeepEqual(d1, d2) || d1.Transactions()[0].PmtId.EndToEndId != "E9999999920261121103000000000000" {
		t.Errorf("Rand: %s", d1.Transactions()[0].PmtId.EndToEndId)
	}
	if _, err := NewBuilder(created).Rand(iotest.ErrReader(errors.New("boom"))).Add(sampleTransfer()).Build(); err == nil {
		t.Error("random source error ignored (MsgId)")
	}
	if _, err := NewBuilder(created).MsgID("m").Rand(iotest.ErrReader(errors.New("boom"))).Add(sampleTransfer()).Build(); err == nil {
		t.Error("random source error ignored (EndToEndId)")
	}
}

func TestBuilderValidationErrors(t *testing.T) {
	tr := sampleTransfer()
	tr.Amount = 0
	tr.Debtor.TaxID = "52998224724"
	tr.CreditorAccount.ISPB = "1234"
	tr.PixKey = "not a key"
	d, err := NewBuilder(time.Now()).Add(tr).Build()
	if d == nil {
		t.Fatal("document not returned with violations")
	}
	var vs xsd.Violations
	if !errors.As(err, &vs) {
		t.Fatalf("want xsd.Violations, got %v", err)
	}
	want := []string{
		root + "/CdtTrfTxInf[0]/IntrBkSttlmAmt: must be greater than zero",
		root + "/CdtTrfTxInf[0]/Dbtr/Id/PrvtId/Othr[0]/Id: must be a valid CPF, got \"52998224724\"",
		root + "/CdtTrfTxInf[0]/CdtrAgt/FinInstnId/ClrSysMmbId/MmbId: must be an 8-digit ISPB, got \"1234\"",
		root + "/CdtTrfTxInf[0]/CdtrAcct/Prxy/Id: \"not a key\" is not a CPF, CNPJ, e-mail, phone or EVP key",
	}
	if len(vs) != len(want) {
		t.Fatalf("got %d violations:\n%v", len(vs), vs)
	}
	for i := range want {
		if vs[i].Error() != want[i] {
			t.Errorf("violation %d:\n got %s\nwant %s", i, vs[i], want[i])
		}
	}
	var v xsd.Violation
	if !errors.As(err, &v) || v.Path == "" {
		t.Error("errors.As(Violation) failed")
	}
	if !strings.HasPrefix(err.Error(), "4 violations:") {
		t.Errorf("Error() = %q", err.Error())
	}

	// Invalid ISPB on the debtor side: no ids are generated, the
	// violations say so.
	tr = sampleTransfer()
	tr.DebtorAccount.ISPB = ""
	_, err = NewBuilder(time.Now()).Add(tr).Build()
	for _, frag := range []string{"GrpHdr/MsgId: is required", "PmtId/EndToEndId: is required", "DbtrAgt/FinInstnId/ClrSysMmbId/MmbId: is required"} {
		if err == nil || !strings.Contains(err.Error(), frag) {
			t.Errorf("missing %q in %v", frag, err)
		}
	}
}

func TestValidateEmpty(t *testing.T) {
	d := &Document{}
	err := d.Validate()
	var vs xsd.Violations
	if !errors.As(err, &vs) {
		t.Fatal(err)
	}
	msgs := err.Error()
	for _, frag := range []string{
		"GrpHdr/MsgId: is required",
		"GrpHdr/CreDtTm: is required",
		"GrpHdr/NbOfTxs: is required",
		"SttlmMtd: is required",
		"CdtTrfTxInf: at least one transaction is required",
	} {
		if !strings.Contains(msgs, frag) {
			t.Errorf("missing %q in\n%s", frag, msgs)
		}
	}
}

func TestValidateRules(t *testing.T) {
	base, err := Parse(load(t, golden))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(d *Document, tx *CreditTransferTransaction)
		want   string
	}{
		{"namespace", func(d *Document, _ *CreditTransferTransaction) { d.XMLName.Space = "urn:x" }, "/Document/@xmlns: must be a pacs.008 namespace"},
		{"msgid too long", func(d *Document, _ *CreditTransferTransaction) {
			d.FIToFICstmrCdtTrf.GrpHdr.MsgId = strings.Repeat("M", 36)
		}, "MsgId: must be at most 35 characters, got 36"},
		{"bad date", func(d *Document, _ *CreditTransferTransaction) { d.FIToFICstmrCdtTrf.GrpHdr.CreDtTm = "21/11/2026" }, "CreDtTm: must be an ISO 8601 date and time"},
		{"nboftxs mismatch", func(d *Document, _ *CreditTransferTransaction) { d.FIToFICstmrCdtTrf.GrpHdr.NbOfTxs = "2" }, "NbOfTxs: is 2 but the message has 1 transactions"},
		{"nboftxs junk", func(d *Document, _ *CreditTransferTransaction) { d.FIToFICstmrCdtTrf.GrpHdr.NbOfTxs = "+1" }, "must be a number of up to 15 digits"},
		{"sttlmmtd", func(d *Document, _ *CreditTransferTransaction) { d.FIToFICstmrCdtTrf.GrpHdr.SttlmInf.SttlmMtd = "INDA" }, `SttlmMtd: must be one of [CLRG], got "INDA"`},
		{"e2e", func(_ *Document, tx *CreditTransferTransaction) { tx.PmtId.EndToEndId = "E123" }, "EndToEndId: must be 32 characters, got 4"},
		{"currency", func(_ *Document, tx *CreditTransferTransaction) { tx.IntrBkSttlmAmt.Ccy = "USD" }, `IntrBkSttlmAmt/@Ccy: must be "BRL", got "USD"`},
		{"negative", func(_ *Document, tx *CreditTransferTransaction) { tx.IntrBkSttlmAmt.Value = -1 }, "IntrBkSttlmAmt: must not be negative"},
		{"huge", func(_ *Document, tx *CreditTransferTransaction) { tx.IntrBkSttlmAmt.Value = money.MaxAmount + 1 }, "IntrBkSttlmAmt: exceeds 18 digits"},
		{"chrgbr", func(_ *Document, tx *CreditTransferTransaction) { tx.ChrgBr = "SHAR" }, "ChrgBr: must be one of [SLEV]"},
		{"no acceptance", func(_ *Document, tx *CreditTransferTransaction) { tx.AccptncDtTm = "" }, "AccptncDtTm: is required"},
		{"both ids", func(_ *Document, tx *CreditTransferTransaction) { tx.Dbtr.Id.OrgId = tx.Cdtr.Id.OrgId }, "Dbtr/Id: must contain exactly one of OrgId or PrvtId, got both"},
		{"empty id", func(_ *Document, tx *CreditTransferTransaction) { tx.Dbtr.Id = &xsd.PartyID{} }, "Dbtr/Id: must contain OrgId or PrvtId"},
		{"no id", func(_ *Document, tx *CreditTransferTransaction) { tx.Cdtr.Id = nil }, "Cdtr/Id: is required (CPF or CNPJ)"},
		{"no othr", func(_ *Document, tx *CreditTransferTransaction) { tx.Cdtr.Id.OrgId.Othr = nil }, "Cdtr/Id/OrgId/Othr: is required (CNPJ)"},
		{"bad cnpj", func(_ *Document, tx *CreditTransferTransaction) { tx.Cdtr.Id.OrgId.Othr[0].Id = "11222333000182" }, "must be a valid CNPJ"},
		{"second othr", func(_ *Document, tx *CreditTransferTransaction) {
			tx.Cdtr.Id.OrgId.Othr = append(tx.Cdtr.Id.OrgId.Othr, xsd.GenericID{Id: "", SchmeNm: &xsd.CodeOrProprietary{}})
		}, "Othr[1]/SchmeNm: must contain exactly one of Cd or Prtry"},
		{"name", func(_ *Document, tx *CreditTransferTransaction) { tx.Dbtr.Nm = "" }, "Dbtr/Nm: is required"},
		{"long name", func(_ *Document, tx *CreditTransferTransaction) { tx.Cdtr.Nm = strings.Repeat("a", 141) }, "Cdtr/Nm: must be at most 140 characters"},
		{"invalid utf8", func(_ *Document, tx *CreditTransferTransaction) { tx.Cdtr.Nm = "\xff" }, "Cdtr/Nm: is not valid UTF-8"},
		{"no debtor account", func(_ *Document, tx *CreditTransferTransaction) { tx.DbtrAcct = nil }, "DbtrAcct: is required"},
		{"no account number", func(_ *Document, tx *CreditTransferTransaction) { tx.DbtrAcct.Id.Othr = nil }, "DbtrAcct/Id/Othr/Id: is required (account number)"},
		{"iban and othr", func(_ *Document, tx *CreditTransferTransaction) { tx.DbtrAcct.Id.IBAN = "BR00" }, "DbtrAcct/Id: must contain exactly one of IBAN or Othr, got both"},
		{"account type", func(_ *Document, tx *CreditTransferTransaction) { tx.DbtrAcct.Tp = xsd.Proprietary("XXXX") }, "DbtrAcct/Tp/Prtry: must be one of [CACC SVGS SLRY TRAN]"},
		{"account type code", func(_ *Document, tx *CreditTransferTransaction) { tx.DbtrAcct.Tp = xsd.Code("XXXX") }, "DbtrAcct/Tp/Cd: must be one of"},
		{"no account type", func(_ *Document, tx *CreditTransferTransaction) { tx.CdtrAcct.Tp = nil }, "CdtrAcct/Tp: is required"},
		{"account ccy", func(_ *Document, tx *CreditTransferTransaction) { tx.CdtrAcct.Ccy = "EUR" }, `CdtrAcct/Ccy: must be "BRL"`},
		{"empty key", func(_ *Document, tx *CreditTransferTransaction) { tx.CdtrAcct.Prxy.Id = "" }, "CdtrAcct/Prxy/Id: is required"},
		{"long branch", func(_ *Document, tx *CreditTransferTransaction) { tx.CdtrAcct.Id.Othr.Issr = strings.Repeat("1", 36) }, "CdtrAcct/Id/Othr/Issr: must be at most 35"},
		{"no agent", func(_ *Document, tx *CreditTransferTransaction) { tx.DbtrAgt = xsd.Agent{} }, "DbtrAgt/FinInstnId/ClrSysMmbId/MmbId: is required (ISPB)"},
		{"clrsysid", func(_ *Document, tx *CreditTransferTransaction) {
			tx.DbtrAgt.FinInstnId.ClrSysMmbId.ClrSysId = &xsd.CodeOrProprietary{Cd: "TOOLONG"}
		}, "DbtrAgt/FinInstnId/ClrSysMmbId/ClrSysId/Cd: must be at most 4 characters"},
		{"lclinstrm both", func(_ *Document, tx *CreditTransferTransaction) { tx.PmtTpInf.LclInstrm.Cd = "X" }, "PmtTpInf/LclInstrm: must contain exactly one of Cd or Prtry, got both"},
		{"purp", func(_ *Document, tx *CreditTransferTransaction) { tx.Purp = xsd.Proprietary(strings.Repeat("p", 36)) }, "Purp/Prtry: must be at most 35"},
		{"ustrd", func(_ *Document, tx *CreditTransferTransaction) { tx.RmtInf.Ustrd = []string{strings.Repeat("x", 141)} }, "RmtInf/Ustrd[0]: must be at most 140"},
		{"txid", func(_ *Document, tx *CreditTransferTransaction) { tx.PmtId.TxId = strings.Repeat("t", 36) }, "PmtId/TxId: must be at most 35"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _ := Parse(mustMarshal(t, base))
			tt.mutate(d, &d.FIToFICstmrCdtTrf.CdtTrfTxInf[0])
			err := d.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func mustMarshal(t testing.TB, d *Document) []byte {
	t.Helper()
	b, err := d.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMarshalDefaultNamespace(t *testing.T) {
	d := &Document{}
	b := mustMarshal(t, d)
	if !bytes.Contains(b, []byte(`<Document xmlns="`+DefaultNamespace+`">`)) {
		t.Fatalf("%s", b)
	}
	if d.XMLName.Space != "" {
		t.Error("Marshal mutated the receiver")
	}
	if d.Version() != "" || d.MessageName() != "pacs.008.001.08" {
		t.Errorf("Version %q MessageName %q", d.Version(), d.MessageName())
	}
	d.FIToFICstmrCdtTrf.CdtTrfTxInf = []CreditTransferTransaction{{IntrBkSttlmAmt: xsd.BRL(-1)}}
	if _, err := d.Marshal(); !errors.Is(err, money.ErrInvalidAmount) {
		t.Errorf("negative amount marshalled: %v", err)
	}
}

func FuzzRoundTrip(f *testing.F) {
	f.Add(load(f, golden))
	f.Add(load(f, "../testdata/envelope_pacs008.xml"))
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Parse(data)
		if err != nil {
			return
		}
		_ = d.Validate() // must not panic
		out, err := d.Marshal()
		if err != nil {
			return // e.g. amounts above MaxAmount cannot be marshalled
		}
		d2, err := Parse(out)
		if err != nil {
			t.Fatalf("re-parse failed: %v\n%s", err, out)
		}
		if !reflect.DeepEqual(d, d2) {
			t.Fatalf("round trip mismatch:\n%#v\n%#v", d, d2)
		}
		out2, err := d2.Marshal()
		if err != nil || !bytes.Equal(out, out2) {
			t.Fatalf("marshal not stable: %v", err)
		}
	})
}

// FuzzBuilderRoundTrip checks the round-trip guarantee from the other
// side: any document the Builder produces and Validate accepts survives
// Parse(Marshal(doc)) unchanged.
func FuzzBuilderRoundTrip(f *testing.F) {
	f.Add("Fulano de Tal", "Loja Exemplo", "Pedido 4521", "pagamentos@example.com", "0001", "123456", "QRDN", "txid123", int64(15075), int64(1795000000))
	f.Add("José <&> \"Ünïcödé\"", " spaced ", "line1\nline2\r\t", "+5521987654321", "", "1", "", "", int64(1), int64(0))
	f.Fuzz(func(t *testing.T, dname, cname, rmt, key, branch, number, initiation, txid string, cents, unix int64) {
		tr := sampleTransfer()
		tr.Debtor.Name, tr.Creditor.Name, tr.RemittanceInfo, tr.PixKey = dname, cname, rmt, key
		tr.DebtorAccount.Branch, tr.DebtorAccount.Number, tr.Initiation, tr.TxID = branch, number, initiation, txid
		tr.Amount = money.Amount(cents)
		tr.AcceptedAt = time.Unix(unix%253402300799, 0)
		d, err := NewBuilder(tr.AcceptedAt).Rand(bytes.NewReader(make([]byte, 256))).Add(tr).Build()
		if err != nil {
			return
		}
		out, err := d.Marshal()
		if err != nil {
			t.Fatalf("valid document does not marshal: %v", err)
		}
		back, err := Parse(out)
		if err != nil {
			t.Fatalf("valid document does not parse back: %v\n%s", err, out)
		}
		if !reflect.DeepEqual(d, back) {
			t.Fatalf("round trip mismatch:\n%#v\n%#v", d, back)
		}
		if err := back.Validate(); err != nil {
			t.Fatalf("re-parsed document no longer validates: %v", err)
		}
	})
}
