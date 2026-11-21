package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/pacs004"
)

func runCmd(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

const td = "../../testdata/"

func TestInspect(t *testing.T) {
	tests := map[string][]string{
		"pacs008.xml":          {"pacs.008.001.08 (FIToFICustomerCreditTransfer)", "E99999999202611211030a1B2c3D4e5F", "150.75 BRL", "Fulano de Tal (52998224725) ISPB 99999999 ag 0001 cc 123456", "pagamentos@example.com", "Pedido 4521", "Valid:           yes"},
		"envelope_pacs008.xml": {"AppHdr:          99999999 -> 00038166"},
		"pacs002_reject.xml":   {"RJCT", "AC03 InvalidCreditorAccountNumber", "Conta do recebedor inexistente"},
		"pacs002_accept.xml":   {"ACSC", "Settled:         2026-11-21T10:30:01.789Z"},
		"pacs004.xml":          {"D88888888202611211500Zx9Yw8Vu7Ts", "50.00 BRL", "MD06 RefundRequestByEndCustomer"},
		"camt054.xml":          {"account 0001/6543210", "CRDT:", "DBIT:", "150.75 BRL  BOOK"},
	}
	for file, wants := range tests {
		code, out, stderr := runCmd(t, "", "inspect", td+file)
		if code != 0 {
			t.Errorf("%s: exit %d: %s", file, code, stderr)
		}
		for _, w := range wants {
			if !strings.Contains(out, w) {
				t.Errorf("%s: output lacks %q:\n%s", file, w, out)
			}
		}
	}
}

func TestInspectStdinAndErrors(t *testing.T) {
	data, _ := os.ReadFile(td + "pacs004.xml")
	if code, out, _ := runCmd(t, string(data), "inspect", "-"); code != 0 || !strings.Contains(out, "pacs.004") {
		t.Errorf("stdin: %d %s", code, out)
	}
	unknown := strings.Replace(string(data), "MD06", "ZZ01", 1)
	if _, out, _ := runCmd(t, unknown, "inspect", "-"); !strings.Contains(out, "ZZ01 (unknown code, kept as-is)") {
		t.Errorf("unknown code: %s", out)
	}
	invalid := strings.Replace(string(data), "<NbOfTxs>1", "<NbOfTxs>2", 1)
	if _, out, _ := runCmd(t, invalid, "inspect", "-"); !strings.Contains(out, "Valid:           no") {
		t.Errorf("invalid: %s", out)
	}
	if code, _, _ := runCmd(t, `<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.08"/>`, "inspect", "-"); code != 1 {
		t.Error("unsupported message accepted")
	}
	if code, _, _ := runCmd(t, "<a/>", "inspect", "-"); code != 1 {
		t.Error("non-message accepted")
	}
	if code, _, _ := runCmd(t, "", "inspect", "does-not-exist.xml"); code != 2 {
		t.Error("missing file")
	}
	if code, _, _ := runCmd(t, "", "inspect"); code != 2 {
		t.Error("missing argument")
	}
}

func TestValidate(t *testing.T) {
	for _, f := range []string{"pacs008.xml", "envelope_pacs008.xml", "pacs002_accept.xml", "pacs002_reject.xml", "pacs004.xml", "camt054.xml"} {
		if code, out, stderr := runCmd(t, "", "validate", td+f); code != 0 || !strings.Contains(out, ": valid") {
			t.Errorf("%s: %d %s %s", f, code, out, stderr)
		}
	}
	data, _ := os.ReadFile(td + "envelope_pacs008.xml")
	bad := strings.Replace(string(data), "<MsgDefIdr>pacs.008.001.08", "<MsgDefIdr>pacs.008.001.09", 1)
	bad = strings.Replace(bad, "<SttlmMtd>CLRG", "<SttlmMtd>INDA", 1)
	code, out, _ := runCmd(t, bad, "validate", "-")
	if code != 1 || !strings.Contains(out, "2 violation(s)") || !strings.Contains(out, "/AppHdr/MsgDefIdr") || !strings.Contains(out, "SttlmMtd") {
		t.Errorf("violations: %d\n%s", code, out)
	}
	if code, _, _ := runCmd(t, "<x", "validate", "-"); code != 1 {
		t.Error("malformed accepted")
	}
}

func TestReturn(t *testing.T) {
	code, out, stderr := runCmd(t, "", "return", "-reason", "MD06", "-amount", "10.00", "-info", "parcial", td+"pacs008.xml")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	d, err := pacs004.Parse([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	tx := d.Transactions()[0]
	if tx.Amount() != money.MustParse("10") || tx.Reason() != "MD06" || tx.RtrRsnInf[0].AddtlInf[0] != "parcial" || d.Validate() != nil {
		t.Errorf("unexpected return:\n%s", out)
	}

	// Full amount by default, minus prior returns.
	_, out, _ = runCmd(t, "", "return", "--reason", "BE08", "--prior", "50.75", "--e2e", "E99999999202611211030a1B2c3D4e5F", td+"pacs008.xml")
	d, err = pacs004.Parse([]byte(out))
	if err != nil || d.Transactions()[0].Amount() != money.MustParse("100") {
		t.Errorf("default amount: %v\n%s", err, out)
	}

	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.xml")
	os.WriteFile(empty, []byte(`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08"/>`), 0o644)

	for _, tt := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"return", "-reason", "MD06", "-amount", "150.76", td + "pacs008.xml"}, 1, "exceeds"},
		{[]string{"return", "-reason", "XX99", td + "pacs008.xml"}, 1, "unknown return reason"},
		{[]string{"return", "-reason", "MD06", "-amount", "1,00", td + "pacs008.xml"}, 2, "-amount"},
		{[]string{"return", "-reason", "MD06", "-prior", "x", td + "pacs008.xml"}, 2, "-prior"},
		{[]string{"return", td + "pacs008.xml"}, 2, "usage"},
		{[]string{"return", "-bogus", td + "pacs008.xml"}, 2, "flag provided but not defined"},
		{[]string{"return", "-reason", "MD06", td + "pacs004.xml"}, 1, "unexpected message type"},
		{[]string{"return", "-reason", "MD06", "nope.xml"}, 2, "nope.xml"},
		{[]string{"return", "-reason", "MD06", empty}, 1, "no transactions"},
	} {
		code, _, stderr := runCmd(t, "", tt.args...)
		if code != tt.code || !strings.Contains(stderr, tt.msg) {
			t.Errorf("%v: exit %d, stderr %q", tt.args, code, stderr)
		}
	}
}

func TestUsage(t *testing.T) {
	if code, _, stderr := runCmd(t, ""); code != 2 || !strings.Contains(stderr, "usage") {
		t.Error("no args")
	}
	if code, out, _ := runCmd(t, "", "help"); code != 0 || !strings.Contains(out, "usage") {
		t.Error("help")
	}
	if code, _, stderr := runCmd(t, "", "frobnicate"); code != 2 || !strings.Contains(stderr, "unknown command") {
		t.Error("unknown command")
	}
}

func TestReadInputLimit(t *testing.T) {
	big := strings.NewReader(strings.Repeat(" ", maxInput+1))
	if _, err := readInput("-", big); err == nil {
		t.Error("oversized input accepted")
	}
}
