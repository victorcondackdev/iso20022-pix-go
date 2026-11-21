package pacs008

import (
	"github.com/victorcondackdev/iso20022-pix-go/internal/check"
	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
)

const root = "/Document/FIToFICstmrCdtTrf"

// Validate checks the document and returns every violation found as
// xsd.Violations, or nil. The rules are:
//
//   - namespace, when set, is a pacs.008 namespace;
//   - GrpHdr: MsgId (Max35Text), CreDtTm (ISODateTime), NbOfTxs equal to
//     the number of CdtTrfTxInf, SttlmMtd "CLRG";
//   - each CdtTrfTxInf: a valid Pix EndToEndId, optional TxId (Max35Text),
//     a positive BRL amount, AccptncDtTm, ChrgBr "SLEV", debtor with name
//     and valid CPF or CNPJ, debtor account (number and type), DbtrAgt
//     and CdtrAgt identified by 8-digit ISPB, creditor with valid CPF or
//     CNPJ, creditor account (number and type, optional Pix key in Prxy
//     that must be a valid key), and well-formed optional PmtTpInf, Purp
//     and RmtInf/Ustrd (Max140Text).
func (d *Document) Validate() error {
	var c check.Checker
	if d.XMLName.Space != "" {
		if id, ok := xmlutil.ParseMessageID(d.XMLName.Space); !ok || id.Base() != "pacs.008" {
			c.Addf("/Document/@xmlns", "must be a pacs.008 namespace, got %q", d.XMLName.Space)
		}
	}
	m := &d.FIToFICstmrCdtTrf
	g := root + "/GrpHdr"
	c.Max35(g+"/MsgId", m.GrpHdr.MsgId, true)
	c.DateTime(g+"/CreDtTm", m.GrpHdr.CreDtTm, true)
	c.NbOfTxs(g+"/NbOfTxs", m.GrpHdr.NbOfTxs, len(m.CdtTrfTxInf))
	c.OneOf(g+"/SttlmInf/SttlmMtd", m.GrpHdr.SttlmInf.SttlmMtd, true, SettlementClearing)
	if len(m.CdtTrfTxInf) == 0 {
		c.Addf(root+"/CdtTrfTxInf", "at least one transaction is required")
	}
	for i := range m.CdtTrfTxInf {
		validateTx(&c, check.Idx(root+"/CdtTrfTxInf", i), &m.CdtTrfTxInf[i])
	}
	return c.Err()
}

func validateTx(c *check.Checker, p string, t *CreditTransferTransaction) {
	c.EndToEndID(p+"/PmtId/EndToEndId", t.PmtId.EndToEndId)
	c.Max35(p+"/PmtId/TxId", t.PmtId.TxId, false)
	if t.PmtTpInf != nil {
		c.Code(p+"/PmtTpInf/InstrPrty", t.PmtTpInf.InstrPrty, false)
		c.Choice(p+"/PmtTpInf/LclInstrm", t.PmtTpInf.LclInstrm, 35)
		c.Choice(p+"/PmtTpInf/CtgyPurp", t.PmtTpInf.CtgyPurp, 35)
	}
	c.BRL(p+"/IntrBkSttlmAmt", t.IntrBkSttlmAmt, true)
	c.DateTime(p+"/AccptncDtTm", t.AccptncDtTm, true)
	c.OneOf(p+"/ChrgBr", t.ChrgBr, true, ChargesSLEV)
	c.Party(p+"/Dbtr", t.Dbtr, true, true)
	c.Account(p+"/DbtrAcct", t.DbtrAcct, true)
	c.Agent(p+"/DbtrAgt", t.DbtrAgt)
	c.Agent(p+"/CdtrAgt", t.CdtrAgt)
	c.Party(p+"/Cdtr", t.Cdtr, false, true)
	c.Account(p+"/CdtrAcct", t.CdtrAcct, true)
	c.Choice(p+"/Purp", t.Purp, 35)
	c.Remittance(p+"/RmtInf", t.RmtInf)
}
