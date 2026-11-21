// Package ids parses, validates and generates the identifiers that appear
// in Pix ISO 20022 messages: EndToEndId, the return identification RtrId,
// message identifications (MsgId), ISPB codes, CPF and CNPJ numbers and Pix
// keys.
//
// EndToEndId and RtrId share one 32-character layout, documented by the
// Banco Central do Brasil in the Pix regulations:
//
//	E 99999999 202611211030 a1B2c3D4e5F   (EndToEndId)
//	D 99999999 202611211030 a1B2c3D4e5F   (RtrId, devolução)
//	│ │        │            └ 11 alphanumeric characters
//	│ │        └ creation time, yyyyMMddHHmm, UTC
//	│ └ ISPB of the participant that created the identifier
//	└ prefix
//
// The identifiers are case-sensitive and are carried verbatim.
package ids

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"
	"unicode/utf8"
)

// Errors wrapped by the functions of this package.
var (
	ErrInvalidEndToEndID = errors.New("ids: invalid EndToEndId")
	ErrInvalidReturnID   = errors.New("ids: invalid RtrId")
	ErrInvalidMsgID      = errors.New("ids: invalid MsgId")
	ErrInvalidISPB       = errors.New("ids: invalid ISPB")
)

// Length is the length of an EndToEndId or RtrId.
const Length = 32

// Prefixes of the 32-character Pix identifiers.
const (
	PrefixEndToEnd byte = 'E'
	PrefixReturn   byte = 'D'
)

const (
	timeLayout  = "200601021504" // yyyyMMddHHmm
	sequenceLen = 11
	alphabet    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// PixID is a parsed EndToEndId or RtrId.
type PixID struct {
	Prefix   byte      // PrefixEndToEnd or PrefixReturn
	ISPB     string    // 8-digit ISPB of the participant that created the id
	Time     time.Time // creation time, UTC, minute precision
	Sequence string    // 11 alphanumeric characters
}

// String returns the 32-character identifier.
func (p PixID) String() string {
	return string(p.Prefix) + p.ISPB + p.Time.UTC().Format(timeLayout) + p.Sequence
}

// ParseEndToEndID parses and validates an EndToEndId. The timestamp must be
// a real calendar date and time. Errors wrap ErrInvalidEndToEndID.
func ParseEndToEndID(s string) (PixID, error) {
	return parse(s, PrefixEndToEnd, ErrInvalidEndToEndID)
}

// ParseReturnID parses and validates a return identification (RtrId) as
// used in pacs.004. Errors wrap ErrInvalidReturnID.
func ParseReturnID(s string) (PixID, error) {
	return parse(s, PrefixReturn, ErrInvalidReturnID)
}

func parse(s string, prefix byte, sentinel error) (PixID, error) {
	if len(s) != Length {
		return PixID{}, fmt.Errorf("%w: must be %d characters, got %d", sentinel, Length, len(s))
	}
	if s[0] != prefix {
		return PixID{}, fmt.Errorf("%w: must start with %q", sentinel, prefix)
	}
	ispb, ts, seq := s[1:9], s[9:21], s[21:]
	if !isDigits(ispb) {
		return PixID{}, fmt.Errorf("%w: ISPB %q must be 8 digits", sentinel, ispb)
	}
	if !isDigits(ts) {
		return PixID{}, fmt.Errorf("%w: timestamp %q must be 12 digits", sentinel, ts)
	}
	t, err := time.Parse(timeLayout, ts)
	if err != nil {
		return PixID{}, fmt.Errorf("%w: timestamp %q is not a valid date and time", sentinel, ts)
	}
	for i := 0; i < len(seq); i++ {
		if !isAlnum(seq[i]) {
			return PixID{}, fmt.Errorf("%w: sequence %q must be alphanumeric", sentinel, seq)
		}
	}
	return PixID{Prefix: prefix, ISPB: ispb, Time: t, Sequence: seq}, nil
}

// NewEndToEndID creates an EndToEndId for the participant with the given
// ISPB at time t (converted to UTC and truncated to the minute). The
// sequence is drawn uniformly from [0-9A-Za-z] using rnd, or crypto/rand
// when rnd is nil.
func NewEndToEndID(ispb string, t time.Time, rnd io.Reader) (string, error) {
	return newID(PrefixEndToEnd, ispb, t, rnd, ErrInvalidEndToEndID)
}

// NewReturnID creates a return identification (RtrId) for the participant
// with the given ISPB at time t. See NewEndToEndID.
func NewReturnID(ispb string, t time.Time, rnd io.Reader) (string, error) {
	return newID(PrefixReturn, ispb, t, rnd, ErrInvalidReturnID)
}

func newID(prefix byte, ispb string, t time.Time, rnd io.Reader, sentinel error) (string, error) {
	if !ValidISPB(ispb) {
		return "", fmt.Errorf("%w: ISPB %q must be 8 digits", sentinel, ispb)
	}
	t = t.UTC()
	if y := t.Year(); y < 0 || y > 9999 {
		return "", fmt.Errorf("%w: year %d out of range", sentinel, y)
	}
	var out [Length]byte
	out[0] = prefix
	copy(out[1:], ispb)
	t.AppendFormat(out[9:9:21], timeLayout)
	if err := randomAlnum(out[21:], rnd); err != nil {
		return "", err
	}
	return string(out[:]), nil
}

// randomAlnum fills dst with characters drawn uniformly from alphabet.
func randomAlnum(dst []byte, rnd io.Reader) error {
	if rnd == nil {
		rnd = rand.Reader
	}
	// Rejection sampling keeps the distribution uniform: 248 = 4 * 62.
	// The attempt cap only matters for a broken reader that keeps
	// returning bytes >= 248.
	var buf [32]byte
	for n, attempts := 0, 0; n < len(dst); attempts++ {
		if attempts == 64 {
			return errors.New("ids: random source yields no usable bytes")
		}
		if _, err := io.ReadFull(rnd, buf[:]); err != nil {
			return fmt.Errorf("ids: reading random bytes: %w", err)
		}
		for _, c := range buf {
			if c < 248 && n < len(dst) {
				dst[n] = alphabet[c%62]
				n++
			}
		}
	}
	return nil
}

// MaxMsgIDLength is the maximum length of an ISO 20022 Max35Text
// identification such as MsgId, BizMsgIdr or TxId.
const MaxMsgIDLength = 35

// ValidateMsgID checks s against the ISO 20022 Max35Text type used by
// MsgId: 1 to 35 characters of valid UTF-8, not blank. It does not enforce
// any SPI-specific layout, which the BCB catalogs define and may change;
// errors wrap ErrInvalidMsgID.
func ValidateMsgID(s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%w: not valid UTF-8", ErrInvalidMsgID)
	}
	n := utf8.RuneCountInString(s)
	if n == 0 || isBlank(s) {
		return fmt.Errorf("%w: empty", ErrInvalidMsgID)
	}
	if n > MaxMsgIDLength {
		return fmt.Errorf("%w: %d characters, maximum is %d", ErrInvalidMsgID, n, MaxMsgIDLength)
	}
	return nil
}

// NewMsgID returns a 32-character message identification made of "M", the
// sender's 8-digit ISPB and 23 random alphanumeric characters drawn from
// rnd (crypto/rand when nil). This is the layout commonly used for SPI
// messages; it always satisfies ValidateMsgID, but check the current BCB
// catalog before relying on the exact layout.
func NewMsgID(ispb string, rnd io.Reader) (string, error) {
	if !ValidISPB(ispb) {
		return "", fmt.Errorf("%w: ISPB %q must be 8 digits", ErrInvalidMsgID, ispb)
	}
	var out [32]byte
	out[0] = 'M'
	copy(out[1:], ispb)
	if err := randomAlnum(out[9:], rnd); err != nil {
		return "", err
	}
	return string(out[:]), nil
}

// ValidISPB reports whether s has the shape of an ISPB: exactly 8 ASCII
// digits. Whether the ISPB belongs to an SPI participant can only be
// checked against the BCB participant list.
func ValidISPB(s string) bool {
	return len(s) == 8 && isDigits(s)
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isBlank(s string) bool {
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
		default:
			return false
		}
	}
	return true
}
