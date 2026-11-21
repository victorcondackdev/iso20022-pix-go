package pacs008

import (
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/ids"
	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// Party is a payer or payee: a name and a CPF (11 digits) or CNPJ (14
// characters). The ISO 20022 identification (PrvtId or OrgId) is chosen
// from the length of TaxID.
type Party struct {
	Name  string
	TaxID string
}

func (p Party) xsd() xsd.Party {
	if len(p.TaxID) == 14 {
		return xsd.Organisation(p.Name, p.TaxID)
	}
	return xsd.Person(p.Name, p.TaxID)
}

// Account is a Pix account: the participant ISPB, the branch (agência;
// empty for payment accounts without branch), the account number and the
// account type (xsd.AccountCurrent, AccountSavings, AccountSalary or
// AccountPayments).
type Account struct {
	ISPB   string
	Branch string
	Number string
	Type   string
}

// Transfer is the flat, Pix-oriented description of one credit transfer
// that the Builder maps onto CdtTrfTxInf.
type Transfer struct {
	// EndToEndID is the Pix EndToEndId. When empty, Build generates one
	// for the debtor ISPB at AcceptedAt.
	EndToEndID string
	// TxID is the QR code transaction id (PmtId/TxId), optional.
	TxID string
	// Amount is the amount in centavos. It must be positive.
	Amount money.Amount
	// AcceptedAt is when the payer's participant accepted the order
	// (AccptncDtTm). Defaults to the builder's creation time.
	AcceptedAt time.Time
	// Initiation is PmtTpInf/LclInstrm/Prtry (e.g. InitiationDICT),
	// optional.
	Initiation string
	// Priority is PmtTpInf/InstrPrty (e.g. "HIGH"), optional.
	Priority string

	Debtor          Party
	DebtorAccount   Account
	Creditor        Party
	CreditorAccount Account
	// PixKey is the creditor's Pix key, carried in CdtrAcct/Prxy/Id.
	PixKey string
	// Purpose is Purp/Cd, optional.
	Purpose string
	// RemittanceInfo is the free-text message for the payee
	// (RmtInf/Ustrd, at most 140 characters), optional.
	RemittanceInfo string
}

// Builder assembles a pacs.008 Document. The zero value is not usable;
// call NewBuilder.
type Builder struct {
	msgID   string
	created time.Time
	rnd     io.Reader
	txs     []Transfer
}

// NewBuilder returns a builder for a message created at the given time.
func NewBuilder(created time.Time) *Builder {
	return &Builder{created: created}
}

// MsgID sets GrpHdr/MsgId. When not set, Build generates one with
// ids.NewMsgID for the debtor ISPB of the first transfer.
func (b *Builder) MsgID(id string) *Builder { b.msgID = id; return b }

// Rand sets the random source used for generated identifiers (crypto/rand
// by default). Useful for deterministic tests.
func (b *Builder) Rand(r io.Reader) *Builder { b.rnd = r; return b }

// Add appends a transfer.
func (b *Builder) Add(t Transfer) *Builder { b.txs = append(b.txs, t); return b }

// Build maps the transfers onto a Document and validates it. On
// validation failure it returns the document together with the
// xsd.Violations, so callers can inspect what was produced.
func (b *Builder) Build() (*Document, error) {
	if len(b.txs) == 0 {
		return nil, errors.New("pacs008: no transfers added")
	}
	d := &Document{XMLName: xmlutil.DocName(DefaultNamespace)}
	m := &d.FIToFICstmrCdtTrf
	m.GrpHdr = GroupHeader{
		MsgId:    b.msgID,
		CreDtTm:  xsd.NewDateTime(b.created),
		NbOfTxs:  strconv.Itoa(len(b.txs)),
		SttlmInf: SettlementInstruction{SttlmMtd: SettlementClearing},
	}
	if m.GrpHdr.MsgId == "" && ids.ValidISPB(b.txs[0].DebtorAccount.ISPB) {
		id, err := ids.NewMsgID(b.txs[0].DebtorAccount.ISPB, b.rnd)
		if err != nil {
			return nil, err
		}
		m.GrpHdr.MsgId = id
	}
	for _, t := range b.txs {
		tx, err := b.transaction(t)
		if err != nil {
			return nil, err
		}
		m.CdtTrfTxInf = append(m.CdtTrfTxInf, tx)
	}
	if err := d.Validate(); err != nil {
		return d, err
	}
	return d, nil
}

func (b *Builder) transaction(t Transfer) (CreditTransferTransaction, error) {
	accepted := t.AcceptedAt
	if accepted.IsZero() {
		accepted = b.created
	}
	e2e := t.EndToEndID
	if e2e == "" && ids.ValidISPB(t.DebtorAccount.ISPB) {
		var err error
		if e2e, err = ids.NewEndToEndID(t.DebtorAccount.ISPB, accepted, b.rnd); err != nil {
			return CreditTransferTransaction{}, err
		}
	}
	tx := CreditTransferTransaction{
		PmtId:          PaymentIdentification{EndToEndId: e2e, TxId: t.TxID},
		IntrBkSttlmAmt: xsd.BRL(t.Amount),
		AccptncDtTm:    xsd.NewDateTime(accepted),
		ChrgBr:         ChargesSLEV,
		Dbtr:           t.Debtor.xsd(),
		DbtrAcct:       xsd.NewAccount(t.DebtorAccount.Branch, t.DebtorAccount.Number, t.DebtorAccount.Type),
		DbtrAgt:        xsd.AgentISPB(t.DebtorAccount.ISPB),
		CdtrAgt:        xsd.AgentISPB(t.CreditorAccount.ISPB),
		Cdtr:           t.Creditor.xsd(),
	}
	if t.Initiation != "" || t.Priority != "" {
		tx.PmtTpInf = &PaymentTypeInformation{InstrPrty: t.Priority}
		if t.Initiation != "" {
			tx.PmtTpInf.LclInstrm = xsd.Proprietary(t.Initiation)
		}
	}
	tx.CdtrAcct = xsd.NewAccount(t.CreditorAccount.Branch, t.CreditorAccount.Number, t.CreditorAccount.Type)
	if t.PixKey != "" {
		tx.CdtrAcct.Prxy = &xsd.Proxy{Id: t.PixKey}
	}
	if t.Purpose != "" {
		tx.Purp = xsd.Code(t.Purpose)
	}
	if t.RemittanceInfo != "" {
		tx.RmtInf = &xsd.RemittanceInformation{Ustrd: []string{t.RemittanceInfo}}
	}
	return tx, nil
}
