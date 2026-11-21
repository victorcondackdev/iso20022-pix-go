// Package money represents Brazilian real amounts exactly, as an integer
// number of centavos, and converts them to and from the decimal lexical form
// used by ISO 20022 amount elements such as IntrBkSttlmAmt.
//
// Floating point is never involved: "0.1" + "0.2" is exactly 30 centavos.
//
// ISO 20022 amounts are xs:decimal values restricted to at most 18 total
// digits and, for currencies with two minor units such as BRL, at most two
// fraction digits. This package accepts amounts up to MaxAmount, the
// largest value whose canonical two-fraction-digit form has 18 digits
// (9999999999999999.99). Larger lexical values such as an 18-digit integer
// are rejected rather than silently wrapped; no real payment comes close.
package money

import (
	"errors"
	"fmt"
)

// ErrInvalidAmount is wrapped by every error returned by Parse and
// UnmarshalText.
var ErrInvalidAmount = errors.New("money: invalid amount")

// ErrOverflow is returned by the checked arithmetic helpers when the result
// does not fit in an int64.
var ErrOverflow = errors.New("money: overflow")

// MaxTotalDigits is the ISO 20022 totalDigits facet of amount types.
const MaxTotalDigits = 18

// MaxFractionDigits is the number of minor units of BRL.
const MaxFractionDigits = 2

// MaxAmount is the largest amount accepted by Parse and MarshalText:
// 9999999999999999.99, i.e. 18 digits in canonical form.
const MaxAmount Amount = 999_999_999_999_999_999

// Amount is an amount of Brazilian reais in centavos. The zero value is
// R$ 0,00. Amount implements encoding.TextMarshaler and
// encoding.TextUnmarshaler, so it can be used directly as the character
// data of an XML element.
type Amount int64

// FromCents returns the amount of the given number of centavos.
func FromCents(c int64) Amount { return Amount(c) }

// Cents returns the amount as a number of centavos.
func (a Amount) Cents() int64 { return int64(a) }

// IsZero reports whether the amount is zero.
func (a Amount) IsZero() bool { return a == 0 }

// IsPositive reports whether the amount is greater than zero.
func (a Amount) IsPositive() bool { return a > 0 }

// Parse parses an ISO 20022 decimal amount such as "10", "10.5", "10.50" or
// "0.01". The accepted grammar is deliberately narrower than xs:decimal:
//
//   - one or more ASCII digits, optionally followed by "." and one or two
//     digits;
//   - no sign, no exponent, no surrounding whitespace, no thousands
//     separators, no "," decimal separator;
//   - at most 18 digits in total (leading zeros count, as in the XSD facet).
//
// Negative amounts do not exist in ISO 20022 amount types; the direction of
// a booking is carried by a separate indicator (CdtDbtInd).
func Parse(s string) (Amount, error) {
	if s == "" {
		return 0, fmt.Errorf("%w: empty", ErrInvalidAmount)
	}
	var (
		cents     int64
		digits    int
		fracDigit = -1 // -1 while in the integer part
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			if fracDigit >= 0 {
				if fracDigit == MaxFractionDigits {
					return 0, fmt.Errorf("%w: %q has more than %d fraction digits", ErrInvalidAmount, s, MaxFractionDigits)
				}
				fracDigit++
			}
			digits++
			if digits > MaxTotalDigits {
				return 0, fmt.Errorf("%w: %q has more than %d digits", ErrInvalidAmount, s, MaxTotalDigits)
			}
			cents = cents*10 + int64(c-'0')
		case c == '.':
			if fracDigit >= 0 {
				return 0, fmt.Errorf("%w: %q has more than one decimal point", ErrInvalidAmount, s)
			}
			if i == 0 {
				return 0, fmt.Errorf("%w: %q has no integer digits", ErrInvalidAmount, s)
			}
			fracDigit = 0
		default:
			return 0, fmt.Errorf("%w: %q contains %q", ErrInvalidAmount, s, c)
		}
	}
	if fracDigit == 0 {
		return 0, fmt.Errorf("%w: %q has no digits after the decimal point", ErrInvalidAmount, s)
	}
	// Scale to centavos, checking the bound before multiplying so the
	// product cannot overflow.
	scale := int64(1)
	for f := max(fracDigit, 0); f < MaxFractionDigits; f++ {
		scale *= 10
	}
	if cents > int64(MaxAmount)/scale {
		return 0, fmt.Errorf("%w: %q exceeds %s", ErrInvalidAmount, s, MaxAmount)
	}
	return Amount(cents * scale), nil
}

// MustParse is like Parse but panics on error. It is intended for tests
// and package-level constants.
func MustParse(s string) Amount {
	a, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return a
}

// String formats the amount in the canonical ISO 20022 form, always with
// two fraction digits: "10.50", "0.01", "1234.00". Negative amounts, which
// can arise from arithmetic, are prefixed with "-"; they are rejected by
// Parse and by MarshalText.
func (a Amount) String() string {
	return string(a.appendTo(nil))
}

func (a Amount) appendTo(b []byte) []byte {
	u := uint64(a)
	if a < 0 {
		b = append(b, '-')
		u = uint64(-(a + 1)) + 1 // safe for math.MinInt64
	}
	var buf [24]byte
	i := len(buf)
	for n := 0; n < 3 || u > 0; n++ {
		if n == MaxFractionDigits {
			i--
			buf[i] = '.'
		}
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	return append(b, buf[i:]...)
}

// MarshalText implements encoding.TextMarshaler. It fails for negative
// amounts, which cannot be represented in an ISO 20022 amount element, and
// for amounts above MaxAmount.
func (a Amount) MarshalText() ([]byte, error) {
	if a < 0 {
		return nil, fmt.Errorf("%w: negative amount %s", ErrInvalidAmount, a)
	}
	if a > MaxAmount {
		return nil, fmt.Errorf("%w: %s exceeds %s", ErrInvalidAmount, a, MaxAmount)
	}
	return a.appendTo(nil), nil
}

// UnmarshalText implements encoding.TextUnmarshaler using Parse.
func (a *Amount) UnmarshalText(b []byte) error {
	v, err := Parse(string(b))
	if err != nil {
		return err
	}
	*a = v
	return nil
}

// Add returns a+b, or ErrOverflow if the sum does not fit in an int64.
func (a Amount) Add(b Amount) (Amount, error) {
	s := a + b
	if (b > 0 && s < a) || (b < 0 && s > a) {
		return 0, ErrOverflow
	}
	return s, nil
}

// Sub returns a-b, or ErrOverflow if the difference does not fit in an
// int64.
func (a Amount) Sub(b Amount) (Amount, error) {
	d := a - b
	if (b > 0 && d > a) || (b < 0 && d < a) {
		return 0, ErrOverflow
	}
	return d, nil
}
