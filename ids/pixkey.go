package ids

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrInvalidPixKey is wrapped by the errors of ClassifyPixKey.
var ErrInvalidPixKey = errors.New("ids: invalid Pix key")

// KeyType is the type of a Pix key (chave Pix) as registered in the DICT.
type KeyType int

// Pix key types.
const (
	KeyUnknown KeyType = iota
	KeyCPF             // 11-digit CPF
	KeyCNPJ            // 14-character CNPJ, numeric or alphanumeric
	KeyEmail           // e-mail address, lowercase, at most 77 characters
	KeyPhone           // E.164 phone number, e.g. +5521987654321
	KeyEVP             // random key: lowercase UUID with hyphens
)

// String returns the DICT name of the key type: CPF, CNPJ, EMAIL, PHONE,
// EVP or UNKNOWN.
func (k KeyType) String() string {
	switch k {
	case KeyCPF:
		return "CPF"
	case KeyCNPJ:
		return "CNPJ"
	case KeyEmail:
		return "EMAIL"
	case KeyPhone:
		return "PHONE"
	case KeyEVP:
		return "EVP"
	default:
		return "UNKNOWN"
	}
}

// MaxEmailKeyLength is the maximum length of an e-mail Pix key.
const MaxEmailKeyLength = 77

// ClassifyPixKey returns the type of a Pix key as it appears in a message
// (for example in CdtrAcct/Prxy/Id). Keys inside SPI messages are already
// in canonical form, so no normalisation is applied: formatted CPFs,
// uppercase e-mails or phone numbers without "+" are rejected.
func ClassifyPixKey(s string) (KeyType, error) {
	switch {
	case s == "":
		return KeyUnknown, fmt.Errorf("%w: empty", ErrInvalidPixKey)
	case strings.IndexByte(s, '@') >= 0:
		if err := checkEmail(s); err != nil {
			return KeyUnknown, err
		}
		return KeyEmail, nil
	case s[0] == '+':
		if !validPhone(s) {
			return KeyUnknown, fmt.Errorf("%w: phone key %q must be E.164 (+ and up to 15 digits)", ErrInvalidPixKey, s)
		}
		return KeyPhone, nil
	case len(s) == 36 && s[8] == '-':
		if !validUUID(s) {
			return KeyUnknown, fmt.Errorf("%w: EVP key %q must be a lowercase UUID", ErrInvalidPixKey, s)
		}
		return KeyEVP, nil
	case len(s) == 11 && ValidCPF(s):
		return KeyCPF, nil
	case len(s) == 14 && ValidCNPJ(s):
		return KeyCNPJ, nil
	}
	return KeyUnknown, fmt.Errorf("%w: %q is not a CPF, CNPJ, e-mail, phone or EVP key", ErrInvalidPixKey, s)
}

func checkEmail(s string) error {
	if utf8.RuneCountInString(s) > MaxEmailKeyLength {
		return fmt.Errorf("%w: e-mail key longer than %d characters", ErrInvalidPixKey, MaxEmailKeyLength)
	}
	local, domain, _ := strings.Cut(s, "@")
	if local == "" || domain == "" || strings.IndexByte(domain, '@') >= 0 ||
		!strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("%w: malformed e-mail key %q", ErrInvalidPixKey, s)
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= ' ' || c >= 0x7f || (c >= 'A' && c <= 'Z') {
			return fmt.Errorf("%w: e-mail key %q must be lowercase ASCII without spaces", ErrInvalidPixKey, s)
		}
	}
	return nil
}

// validPhone checks the E.164 shape used by the DICT: "+", a non-zero
// first digit and at most 15 digits in total.
func validPhone(s string) bool {
	d := s[1:]
	return len(d) >= 2 && len(d) <= 15 && d[0] != '0' && isDigits(d)
}

func validUUID(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	return true
}
