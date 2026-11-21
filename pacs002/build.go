package pacs002

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/victorcondackdev/iso20022-pix-go/ids"
	"github.com/victorcondackdev/iso20022-pix-go/internal/check"
	"github.com/victorcondackdev/iso20022-pix-go/internal/xmlutil"
	"github.com/victorcondackdev/iso20022-pix-go/pacs008"
	"github.com/victorcondackdev/iso20022-pix-go/xsd"
)

// ErrUnknownReason is returned by Reject for a reason code that is not in
// Reasons, unless AllowUnknownReason is given.
var ErrUnknownReason = errors.New("pacs002: unknown reason code")

// Option configures Accept and Reject.
type Option func(*options)

type options struct {
	at           time.Time
	settledAt    time.Time
	msgID        string
	ispb         string
	rnd          io.Reader
	addtlInf     []string
	allowUnknown bool
}

// At sets GrpHdr/CreDtTm. Defaults to time.Now.
func At(t time.Time) Option { return func(o *options) { o.at = t } }

// SettledAt sets FctvIntrBkSttlmDt/DtTm on accepted transactions.
func SettledAt(t time.Time) Option { return func(o *options) { o.settledAt = t } }

// WithMsgID sets GrpHdr/MsgId. By default one is generated with
// ids.NewMsgID for the reporting ISPB.
func WithMsgID(id string) Option { return func(o *options) { o.msgID = id } }

// WithISPB sets the ISPB of the reporting participant used to generate the
// MsgId. Defaults to the CdtrAgt of the first original transaction.
func WithISPB(ispb string) Option { return func(o *options) { o.ispb = ispb } }

// WithRand sets the random source for generated identifiers.
func WithRand(r io.Reader) Option { return func(o *options) { o.rnd = r } }

// WithAdditionalInfo adds StsRsnInf/AddtlInf lines (Max105Text) to
// rejections.
func WithAdditionalInfo(lines ...string) Option {
	return func(o *options) { o.addtlInf = append(o.addtlInf, lines...) }
}

// AllowUnknownReason lets Reject use a code that is not in Reasons.
func AllowUnknownReason() Option { return func(o *options) { o.allowUnknown = true } }

// Accept builds a status report accepting every transaction of original
// with TxSts ACSC.
func Accept(original *pacs008.Document, opts ...Option) (*Document, error) {
	return report(original, StatusAcceptedSettlementCompleted, "", opts)
}

// Reject builds a status report rejecting every transaction of original
// with TxSts RJCT and the given reason code (StsRsnInf/Rsn/Cd).
func Reject(original *pacs008.Document, reason string, opts ...Option) (*Document, error) {
	return report(original, StatusRejected, reason, opts)
}

func report(orig *pacs008.Document, status, reason string, opts []Option) (*Document, error) {
	if orig == nil || len(orig.Transactions()) == 0 {
		return nil, errors.New("pacs002: original pacs.008 has no transactions")
	}
	o := options{at: time.Now()}
	for _, opt := range opts {
		opt(&o)
	}
	if status == StatusRejected {
		if reason == "" {
			return nil, errors.New("pacs002: a rejection needs a reason code")
		}
		if _, ok := Reasons.Lookup(reason); !ok && !o.allowUnknown {
			return nil, fmt.Errorf("%w: %q", ErrUnknownReason, reason)
		}
	}
	txs := orig.Transactions()
	if o.ispb == "" {
		o.ispb = txs[0].CdtrAgt.ISPB()
	}
	if o.msgID == "" {
		id, err := ids.NewMsgID(o.ispb, o.rnd)
		if err != nil {
			return nil, err
		}
		o.msgID = id
	}
	d := &Document{XMLName: xmlutil.DocName(DefaultNamespace)}
	r := &d.FIToFIPmtStsRpt
	r.GrpHdr = GroupHeader{MsgId: o.msgID, CreDtTm: xsd.NewDateTime(o.at)}
	h := orig.FIToFICstmrCdtTrf.GrpHdr
	r.OrgnlGrpInfAndSts = []OriginalGroupHeader{{
		OrgnlMsgId:   h.MsgId,
		OrgnlMsgNmId: orig.MessageName(),
		OrgnlCreDtTm: h.CreDtTm,
	}}
	for _, tx := range txs {
		ts := TransactionStatus{OrgnlEndToEndId: tx.PmtId.EndToEndId, TxSts: status}
		if status == StatusRejected {
			ts.StsRsnInf = []StatusReasonInfo{{Rsn: xsd.Code(reason), AddtlInf: o.addtlInf}}
		} else if !o.settledAt.IsZero() {
			ts.FctvIntrBkSttlmDt = &xsd.DateAndDateTime{DtTm: xsd.NewDateTime(o.settledAt)}
		}
		r.TxInfAndSts = append(r.TxInfAndSts, ts)
	}
	if err := d.Validate(); err != nil {
		return d, err
	}
	return d, nil
}

const root = "/Document/FIToFIPmtStsRpt"

// Validate checks the report and returns every violation as
// xsd.Violations, or nil: MsgId and CreDtTm present, at least one
// TxInfAndSts, a valid Pix OrgnlEndToEndId, a status code, at least one
// reason on rejections, and well-formed optional elements. Reason codes
// are not restricted to Reasons.
func (d *Document) Validate() error {
	var c check.Checker
	if d.XMLName.Space != "" {
		if id, ok := xmlutil.ParseMessageID(d.XMLName.Space); !ok || id.Base() != "pacs.002" {
			c.Addf("/Document/@xmlns", "must be a pacs.002 namespace, got %q", d.XMLName.Space)
		}
	}
	r := &d.FIToFIPmtStsRpt
	c.Max35(root+"/GrpHdr/MsgId", r.GrpHdr.MsgId, true)
	c.DateTime(root+"/GrpHdr/CreDtTm", r.GrpHdr.CreDtTm, true)
	for i, g := range r.OrgnlGrpInfAndSts {
		p := check.Idx(root+"/OrgnlGrpInfAndSts", i)
		c.Max35(p+"/OrgnlMsgId", g.OrgnlMsgId, true)
		c.Max35(p+"/OrgnlMsgNmId", g.OrgnlMsgNmId, true)
		c.DateTime(p+"/OrgnlCreDtTm", g.OrgnlCreDtTm, false)
		c.Code(p+"/GrpSts", g.GrpSts, false)
	}
	if len(r.TxInfAndSts) == 0 {
		c.Addf(root+"/TxInfAndSts", "at least one transaction status is required")
	}
	for i := range r.TxInfAndSts {
		t := &r.TxInfAndSts[i]
		p := check.Idx(root+"/TxInfAndSts", i)
		c.Max35(p+"/StsId", t.StsId, false)
		c.EndToEndID(p+"/OrgnlEndToEndId", t.OrgnlEndToEndId)
		c.Max35(p+"/OrgnlTxId", t.OrgnlTxId, false)
		c.Code(p+"/TxSts", t.TxSts, true)
		if t.TxSts == StatusRejected && t.Reason() == "" {
			c.Addf(p+"/StsRsnInf", "a rejection (RJCT) requires a reason code")
		}
		for j, s := range t.StsRsnInf {
			q := check.Idx(p+"/StsRsnInf", j)
			c.Choice(q+"/Rsn", s.Rsn, 35)
			for k, a := range s.AddtlInf {
				c.Text(check.Idx(q+"/AddtlInf", k), a, 105, true)
			}
		}
		c.DateOrDateTime(p+"/FctvIntrBkSttlmDt", t.FctvIntrBkSttlmDt, false)
	}
	return c.Err()
}
