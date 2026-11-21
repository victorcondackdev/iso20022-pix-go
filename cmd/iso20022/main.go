// Command iso20022 inspects, validates and builds returns for Pix ISO 20022
// messages.
//
// Usage:
//
//	iso20022 inspect  FILE            detect the message and print key fields
//	iso20022 validate FILE            validate and list every violation
//	iso20022 return [flags] FILE      build a pacs.004 return for a pacs.008
//
// FILE may be "-" for standard input. The return command writes the
// pacs.004 to standard output and accepts:
//
//	-reason CODE    return reason (BE08, FR01, MD06, SL02); required
//	-amount VALUE   amount to return, e.g. 10.00; defaults to the full amount
//	-prior VALUE    total already returned for the payment
//	-info TEXT      additional information (RtrRsnInf/AddtlInf)
//	-e2e ID         EndToEndId of the transaction to return
//
// Exit status is 0 on success, 1 when a message is invalid and 2 on usage
// or I/O errors.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	iso20022 "github.com/victorcondackdev/iso20022-pix-go"
	"github.com/victorcondackdev/iso20022-pix-go/camt054"
	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/pacs002"
	"github.com/victorcondackdev/iso20022-pix-go/pacs004"
	"github.com/victorcondackdev/iso20022-pix-go/pacs008"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// maxInput bounds the size of a message read by the tool.
const maxInput = 16 << 20

const usage = `usage:
  iso20022 inspect  FILE
  iso20022 validate FILE
  iso20022 return -reason CODE [-amount VALUE] [-prior VALUE] [-info TEXT] [-e2e ID] FILE
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "inspect":
		return withFile(args[1:], stdin, stderr, func(data []byte) int { return inspect(data, stdout, stderr) })
	case "validate":
		return withFile(args[1:], stdin, stderr, func(data []byte) int { return validate(data, stdout, stderr) })
	case "return":
		return returnCmd(args[1:], stdin, stdout, stderr)
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "iso20022: unknown command %q\n%s", args[0], usage)
	return 2
}

func withFile(args []string, stdin io.Reader, stderr io.Writer, f func([]byte) int) int {
	if len(args) != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	data, err := readInput(args[0], stdin)
	if err != nil {
		fmt.Fprintln(stderr, "iso20022:", err)
		return 2
	}
	return f(data)
}

func readInput(name string, stdin io.Reader) ([]byte, error) {
	r := stdin
	if name != "-" {
		f, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	data, err := io.ReadAll(io.LimitReader(r, maxInput+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxInput {
		return nil, fmt.Errorf("input larger than %d bytes", maxInput)
	}
	return data, nil
}

// message is the common behaviour of the parsed documents.
type message interface{ Validate() error }

func parse(data []byte) (iso20022.Envelope, message, error) {
	env, err := iso20022.Inspect(data)
	if err != nil {
		return env, nil, err
	}
	var m message
	switch env.Type.ID() {
	case iso20022.Pacs008:
		m, err = pacs008.Parse(data)
	case iso20022.Pacs002:
		m, err = pacs002.Parse(data)
	case iso20022.Pacs004:
		m, err = pacs004.Parse(data)
	case iso20022.Camt054:
		m, err = camt054.Parse(data)
	default:
		err = fmt.Errorf("%s is not supported", env.Type)
	}
	return env, m, err
}

func inspect(data []byte, stdout, stderr io.Writer) int {
	env, m, err := parse(data)
	if err != nil {
		fmt.Fprintln(stderr, "iso20022:", err)
		return 1
	}
	w := &printer{w: stdout}
	w.kv("Message", fmt.Sprintf("%s (%s)", env.Type, env.Type.Name()))
	if h := env.Header; h != nil {
		w.kv("AppHdr", fmt.Sprintf("%s -> %s  BizMsgIdr=%s  MsgDefIdr=%s  CreDt=%s", h.Fr.ISPB(), h.To.ISPB(), h.BizMsgIdr, h.MsgDefIdr, h.CreDt))
	}
	switch d := m.(type) {
	case *pacs008.Document:
		g := d.FIToFICstmrCdtTrf.GrpHdr
		w.kv("MsgId", g.MsgId)
		w.kv("CreDtTm", string(g.CreDtTm))
		w.kv("NbOfTxs", g.NbOfTxs)
		for i, t := range d.Transactions() {
			w.section("Transaction %d", i)
			w.kv("EndToEndId", t.PmtId.EndToEndId)
			w.opt("TxId", t.PmtId.TxId)
			w.kv("Amount", t.IntrBkSttlmAmt.String())
			w.kv("Accepted", string(t.AccptncDtTm))
			w.opt("Initiation", t.Initiation())
			w.kv("Debtor", fmt.Sprintf("%s (%s) ISPB %s ag %s cc %s", t.Dbtr.Nm, t.Dbtr.TaxID(), t.DbtrAgt.ISPB(), t.DbtrAcct.Branch(), t.DbtrAcct.Number()))
			w.kv("Creditor", fmt.Sprintf("%s (%s) ISPB %s ag %s cc %s", t.Cdtr.Nm, t.Cdtr.TaxID(), t.CdtrAgt.ISPB(), t.CdtrAcct.Branch(), t.CdtrAcct.Number()))
			w.opt("PixKey", t.CdtrAcct.PixKey())
			w.opt("Remittance", t.RemittanceInfo())
		}
	case *pacs002.Document:
		g := d.FIToFIPmtStsRpt.GrpHdr
		w.kv("MsgId", g.MsgId)
		w.kv("CreDtTm", string(g.CreDtTm))
		for _, o := range d.FIToFIPmtStsRpt.OrgnlGrpInfAndSts {
			w.kv("Original", o.OrgnlMsgId+" ("+o.OrgnlMsgNmId+")")
		}
		for i, t := range d.Transactions() {
			w.section("Transaction %d", i)
			w.kv("OrgnlEndToEndId", t.OrgnlEndToEndId)
			w.kv("Status", t.TxSts)
			if r := t.Reason(); r != "" {
				w.kv("Reason", describe(pacs002.Reasons, r))
			}
			for _, s := range t.StsRsnInf {
				for _, a := range s.AddtlInf {
					w.kv("AddtlInf", a)
				}
			}
			if s := t.FctvIntrBkSttlmDt; s != nil {
				w.kv("Settled", string(s.DtTm)+string(s.Dt))
			}
		}
	case *pacs004.Document:
		g := d.PmtRtr.GrpHdr
		w.kv("MsgId", g.MsgId)
		w.kv("CreDtTm", string(g.CreDtTm))
		for i, t := range d.Transactions() {
			w.section("Transaction %d", i)
			w.kv("RtrId", t.RtrId)
			w.kv("OrgnlEndToEndId", t.OrgnlEndToEndId)
			w.kv("Returned", t.RtrdIntrBkSttlmAmt.String())
			w.kv("Reason", describe(pacs004.Reasons, t.Reason()))
		}
	case *camt054.Document:
		g := d.BkToCstmrDbtCdtNtfctn.GrpHdr
		w.kv("MsgId", g.MsgId)
		w.kv("CreDtTm", string(g.CreDtTm))
		for _, n := range d.BkToCstmrDbtCdtNtfctn.Ntfctn {
			w.section("Notification %s, account %s/%s", n.Id, n.Acct.Branch(), n.Acct.Number())
			for _, e := range n.Ntry {
				booked := ""
				if e.BookgDt != nil {
					booked = string(e.BookgDt.DtTm) + string(e.BookgDt.Dt)
				}
				w.kv(e.CdtDbtInd, fmt.Sprintf("%s  %s  %s  %s", e.Amt, e.Sts.Value(), booked, strings.Join(e.EndToEndIDs(), ",")))
			}
		}
	}
	if err := m.Validate(); err != nil {
		w.kv("Valid", "no (run validate for details)")
	} else {
		w.kv("Valid", "yes")
	}
	return 0
}

func describe(set xsd.CodeSet, code string) string {
	if info, ok := set.Lookup(code); ok {
		return fmt.Sprintf("%s %s: %s / %s", code, info.Name, info.English, info.Portuguese)
	}
	return code + " (unknown code, kept as-is)"
}

func validate(data []byte, stdout, stderr io.Writer) int {
	env, m, err := parse(data)
	if err != nil {
		fmt.Fprintln(stderr, "iso20022:", err)
		return 1
	}
	errs := []error{env.Validate(), m.Validate()}
	var all xsd.Violations
	for _, e := range errs {
		var vs xsd.Violations
		if errors.As(e, &vs) {
			all = append(all, vs...)
		}
	}
	if len(all) == 0 {
		fmt.Fprintf(stdout, "%s: valid\n", env.Type)
		return 0
	}
	fmt.Fprintf(stdout, "%s: %d violation(s)\n", env.Type, len(all))
	for _, v := range all {
		fmt.Fprintln(stdout, "  "+v.Error())
	}
	return 1
}

func returnCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("return", flag.ContinueOnError)
	fs.SetOutput(stderr)
	reason := fs.String("reason", "", "return reason code (BE08, FR01, MD06, SL02)")
	amount := fs.String("amount", "", "amount to return, e.g. 10.00 (default: full amount)")
	prior := fs.String("prior", "", "total already returned for the payment")
	info := fs.String("info", "", "additional information (RtrRsnInf/AddtlInf)")
	e2e := fs.String("e2e", "", "EndToEndId of the transaction to return")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || *reason == "" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	data, err := readInput(fs.Arg(0), stdin)
	if err != nil {
		fmt.Fprintln(stderr, "iso20022:", err)
		return 2
	}
	orig, err := pacs008.Parse(data)
	if err != nil {
		fmt.Fprintln(stderr, "iso20022:", err)
		return 1
	}
	var opts []pacs004.Option
	if *e2e != "" {
		opts = append(opts, pacs004.ForEndToEndID(*e2e))
	}
	if *info != "" {
		opts = append(opts, pacs004.WithAdditionalInfo(*info))
	}
	if *prior != "" {
		p, err := money.Parse(*prior)
		if err != nil {
			fmt.Fprintln(stderr, "iso20022: -prior:", err)
			return 2
		}
		opts = append(opts, pacs004.WithPriorReturns(p))
	}
	var amt money.Amount
	if *amount == "" {
		if len(orig.Transactions()) == 0 {
			fmt.Fprintln(stderr, "iso20022: pacs.008 has no transactions")
			return 1
		}
		amt = orig.Transactions()[0].Amount()
		for _, t := range orig.Transactions() {
			if t.PmtId.EndToEndId == *e2e {
				amt = t.Amount()
			}
		}
		if *prior != "" {
			p, _ := money.Parse(*prior)
			amt -= p
		}
	} else if amt, err = money.Parse(*amount); err != nil {
		fmt.Fprintln(stderr, "iso20022: -amount:", err)
		return 2
	}
	ret, err := pacs004.FromPacs008(orig, *reason, amt, opts...)
	if err != nil {
		fmt.Fprintln(stderr, "iso20022:", err)
		return 1
	}
	out, err := ret.Marshal()
	if err != nil {
		fmt.Fprintln(stderr, "iso20022:", err)
		return 1
	}
	stdout.Write(out)
	return 0
}

type printer struct{ w io.Writer }

func (p *printer) kv(k, v string) { fmt.Fprintf(p.w, "%-16s %s\n", k+":", v) }

func (p *printer) opt(k, v string) {
	if v != "" {
		p.kv(k, v)
	}
}

func (p *printer) section(format string, args ...any) {
	fmt.Fprintf(p.w, "-- "+format+"\n", args...)
}
