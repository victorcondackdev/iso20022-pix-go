package pacs004

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/ids"
	"github.com/victorcondackdev/iso20022-pix-go/internal/check"
	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/money"
	"github.com/victorcondackdev/iso20022-pix-go/pacs008"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// Errors returned by FromPacs008.
var (
	ErrUnknownReason  = errors.New("pacs004: unknown return reason code")
	ErrAmountExceeded = errors.New("pacs004: return amount exceeds the amount available for return")
	ErrNoTransaction  = errors.New("pacs004: original transaction not found")
)

// Option configures FromPacs008.
type Option func(*options)

type options struct {
	at           time.Time
	msgID        string
	returnID     string
	e2e          string
	rnd          io.Reader
	addtlInf     []string
	prior        money.Amount
	allowUnknown bool
}

// At sets the creation time used for GrpHdr/CreDtTm and the generated
// RtrId. Defaults to time.Now.
func At(t time.Time) Option { return func(o *options) { o.at = t } }

// WithMsgID sets GrpHdr/MsgId instead of generating one.
func WithMsgID(id string) Option { return func(o *options) { o.msgID = id } }

// WithReturnID sets TxInf/RtrId instead of generating one.
func WithReturnID(id string) Option { return func(o *options) { o.returnID = id } }

// ForEndToEndID selects the original transaction by EndToEndId when the
// pacs.008 carries more than one. By default the first is used.
func ForEndToEndID(e2e string) Option { return func(o *options) { o.e2e = e2e } }

// WithRand sets the random source for generated identifiers.
func WithRand(r io.Reader) Option { return func(o *options) { o.rnd = r } }

// WithAdditionalInfo adds RtrRsnInf/AddtlInf lines (Max105Text).
func WithAdditionalInfo(lines ...string) Option {
	return func(o *options) { o.addtlInf = append(o.addtlInf, lines...) }
}

// WithPriorReturns declares the total already returned for the original
// payment, so that the sum of partial returns never exceeds it.
func WithPriorReturns(total money.Amount) Option { return func(o *options) { o.prior = total } }

// AllowUnknownReason lets FromPacs008 use a code that is not in Reasons.
func AllowUnknownReason() Option { return func(o *options) { o.allowUnknown = true } }

// FromPacs008 builds a return of amount for a transaction of original. The
// return is issued by the creditor's participant: its ISPB (CdtrAgt) is
// used for the generated MsgId and RtrId. Partial returns are allowed;
// amount plus WithPriorReturns must not exceed the original
// IntrBkSttlmAmt (ErrAmountExceeded).
func FromPacs008(original *pacs008.Document, reason string, amount money.Amount, opts ...Option) (*Document, error) {
	o := options{at: time.Now()}
	for _, opt := range opts {
		opt(&o)
	}
	if original == nil {
		return nil, ErrNoTransaction
	}
	tx, err := pick(original, o.e2e)
	if err != nil {
		return nil, err
	}
	if _, ok := Reasons.Lookup(reason); !ok && !o.allowUnknown {
		return nil, fmt.Errorf("%w: %q", ErrUnknownReason, reason)
	}
	if !amount.IsPositive() {
		return nil, fmt.Errorf("pacs004: return amount must be positive, got %s", amount)
	}
	if o.prior < 0 {
		return nil, fmt.Errorf("pacs004: prior returns must not be negative, got %s", o.prior)
	}
	paid := tx.Amount()
	available, err := paid.Sub(o.prior)
	if err != nil || available < 0 {
		return nil, fmt.Errorf("%w: prior returns %s exceed the original %s", ErrAmountExceeded, o.prior, paid)
	}
	if amount > available {
		return nil, fmt.Errorf("%w: %s requested, %s available of the original %s", ErrAmountExceeded, amount, available, paid)
	}

	ispb := tx.CdtrAgt.ISPB()
	if o.msgID == "" {
		if o.msgID, err = ids.NewMsgID(ispb, o.rnd); err != nil {
			return nil, err
		}
	}
	if o.returnID == "" {
		if o.returnID, err = ids.NewReturnID(ispb, o.at, o.rnd); err != nil {
			return nil, err
		}
	}
	h := original.FIToFICstmrCdtTrf.GrpHdr
	d := &Document{XMLName: xmlutil.DocName(DefaultNamespace)}
	d.PmtRtr.GrpHdr = GroupHeader{
		MsgId:    o.msgID,
		CreDtTm:  xsd.NewDateTime(o.at),
		NbOfTxs:  "1",
		SttlmInf: SettlementInstruction{SttlmMtd: pacs008.SettlementClearing},
	}
	d.PmtRtr.TxInf = []ReturnTransaction{{
		RtrId: o.returnID,
		OrgnlGrpInf: &OriginalGroupInformation{
			OrgnlMsgId:   h.MsgId,
			OrgnlMsgNmId: original.MessageName(),
			OrgnlCreDtTm: h.CreDtTm,
		},
		OrgnlEndToEndId:    tx.PmtId.EndToEndId,
		RtrdIntrBkSttlmAmt: xsd.BRL(amount),
		ChrgBr:             pacs008.ChargesSLEV,
		RtrRsnInf:          []ReturnReasonInfo{{Rsn: xsd.Code(reason), AddtlInf: o.addtlInf}},
	}}
	if err := d.Validate(); err != nil {
		return d, err
	}
	return d, nil
}

func pick(d *pacs008.Document, e2e string) (*pacs008.CreditTransferTransaction, error) {
	txs := d.Transactions()
	if len(txs) == 0 {
		return nil, ErrNoTransaction
	}
	if e2e == "" {
		return &txs[0], nil
	}
	for i := range txs {
		if txs[i].PmtId.EndToEndId == e2e {
			return &txs[i], nil
		}
	}
	return nil, fmt.Errorf("%w: EndToEndId %q", ErrNoTransaction, e2e)
}

const root = "/Document/PmtRtr"

// Validate checks the return and returns every violation as
// xsd.Violations, or nil: GrpHdr as in pacs.008 (MsgId, CreDtTm, NbOfTxs
// matching, SttlmMtd CLRG), and for each TxInf a valid RtrId and
// OrgnlEndToEndId, a positive BRL returned amount not above
// OrgnlIntrBkSttlmAmt when that is present, ChrgBr SLEV when present, and
// at least one reason code. Reason codes are not restricted to Reasons.
func (d *Document) Validate() error {
	var c check.Checker
	if d.XMLName.Space != "" {
		if id, ok := xmlutil.ParseMessageID(d.XMLName.Space); !ok || id.Base() != "pacs.004" {
			c.Addf("/Document/@xmlns", "must be a pacs.004 namespace, got %q", d.XMLName.Space)
		}
	}
	r := &d.PmtRtr
	g := root + "/GrpHdr"
	c.Max35(g+"/MsgId", r.GrpHdr.MsgId, true)
	c.DateTime(g+"/CreDtTm", r.GrpHdr.CreDtTm, true)
	c.NbOfTxs(g+"/NbOfTxs", r.GrpHdr.NbOfTxs, len(r.TxInf))
	c.OneOf(g+"/SttlmInf/SttlmMtd", r.GrpHdr.SttlmInf.SttlmMtd, true, pacs008.SettlementClearing)
	if len(r.TxInf) == 0 {
		c.Addf(root+"/TxInf", "at least one transaction is required")
	}
	for i := range r.TxInf {
		t := &r.TxInf[i]
		p := check.Idx(root+"/TxInf", i)
		c.ReturnID(p+"/RtrId", t.RtrId)
		if g := t.OrgnlGrpInf; g != nil {
			c.Max35(p+"/OrgnlGrpInf/OrgnlMsgId", g.OrgnlMsgId, true)
			c.Max35(p+"/OrgnlGrpInf/OrgnlMsgNmId", g.OrgnlMsgNmId, true)
			c.DateTime(p+"/OrgnlGrpInf/OrgnlCreDtTm", g.OrgnlCreDtTm, false)
		}
		c.EndToEndID(p+"/OrgnlEndToEndId", t.OrgnlEndToEndId)
		c.Max35(p+"/OrgnlTxId", t.OrgnlTxId, false)
		if t.OrgnlIntrBkSttlmAmt != nil {
			c.BRL(p+"/OrgnlIntrBkSttlmAmt", *t.OrgnlIntrBkSttlmAmt, true)
			if t.RtrdIntrBkSttlmAmt.Value > t.OrgnlIntrBkSttlmAmt.Value {
				c.Addf(p+"/RtrdIntrBkSttlmAmt", "%s exceeds OrgnlIntrBkSttlmAmt %s", t.RtrdIntrBkSttlmAmt.Value, t.OrgnlIntrBkSttlmAmt.Value)
			}
		}
		c.BRL(p+"/RtrdIntrBkSttlmAmt", t.RtrdIntrBkSttlmAmt, true)
		c.OneOf(p+"/ChrgBr", t.ChrgBr, false, pacs008.ChargesSLEV)
		if t.Reason() == "" {
			c.Addf(p+"/RtrRsnInf", "a return reason code is required")
		}
		for j, s := range t.RtrRsnInf {
			q := check.Idx(p+"/RtrRsnInf", j)
			c.Choice(q+"/Rsn", s.Rsn, 35)
			for k, a := range s.AddtlInf {
				c.Text(check.Idx(q+"/AddtlInf", k), a, 105, true)
			}
		}
	}
	return c.Err()
}
