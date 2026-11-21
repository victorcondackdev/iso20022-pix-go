package ids

import "strings"

// ValidCPF reports whether s is a valid CPF in the unformatted 11-digit form
// carried in ISO 20022 messages (PrvtId/Othr/Id), with correct check digits.
// Sequences of a single repeated digit, which pass the arithmetic but are
// not issued, are rejected.
func ValidCPF(s string) bool {
	if len(s) != 11 || !isDigits(s) || repeated(s) {
		return false
	}
	return checkDigit(s[:9], 10) == s[9] && checkDigit(s[:10], 11) == s[10]
}

// checkDigit computes a CPF check digit with descending weights starting
// at w.
func checkDigit(s string, w int) byte {
	sum := 0
	for i := 0; i < len(s); i++ {
		sum += int(s[i]-'0') * (w - i)
	}
	r := sum % 11
	if r < 2 {
		return '0'
	}
	return byte('0' + 11 - r)
}

// ValidCNPJ reports whether s is a valid CNPJ in the unformatted
// 14-character form carried in ISO 20022 messages (OrgId/Othr/Id). Both the
// traditional numeric CNPJ and the alphanumeric CNPJ introduced by the
// Receita Federal in 2026 are accepted: the first 12 characters may be
// digits or uppercase letters, the last two are numeric check digits.
func ValidCNPJ(s string) bool {
	if len(s) != 14 || repeated(s) {
		return false
	}
	for i := 0; i < 12; i++ {
		c := s[i]
		if !(c >= '0' && c <= '9') && !(c >= 'A' && c <= 'Z') {
			return false
		}
	}
	if !isDigits(s[12:]) {
		return false
	}
	return cnpjDigit(s[:12]) == s[12] && cnpjDigit(s[:13]) == s[13]
}

// cnpjDigit computes a CNPJ check digit. Each character is valued as its
// ASCII code minus 48, which maps digits to themselves and letters to
// 17..42, per the Receita Federal specification for alphanumeric CNPJ.
func cnpjDigit(s string) byte {
	sum, w := 0, 2
	for i := len(s) - 1; i >= 0; i-- {
		sum += int(s[i]-'0') * w
		w++
		if w > 9 {
			w = 2
		}
	}
	r := sum % 11
	if r < 2 {
		return '0'
	}
	return byte('0' + 11 - r)
}

func repeated(s string) bool {
	return strings.Count(s, s[:1]) == len(s)
}
