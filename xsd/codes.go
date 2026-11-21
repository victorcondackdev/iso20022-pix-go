package xsd

import "sort"

// CodeInfo describes an ISO 20022 external code, such as a status or
// return reason, with its ISO name and English and Portuguese
// descriptions.
type CodeInfo struct {
	Code       string
	Name       string // ISO 20022 code name, e.g. "InsufficientFunds"
	English    string
	Portuguese string
}

// CodeSet is a lookup table of external codes.
type CodeSet map[string]CodeInfo

// Lookup returns the description of code. Unknown codes are reported with
// ok false; messages carrying them are still valid and the code is kept
// as-is.
func (s CodeSet) Lookup(code string) (info CodeInfo, ok bool) {
	info, ok = s[code]
	return info, ok
}

// Sorted returns the entries ordered by code.
func (s CodeSet) Sorted() []CodeInfo {
	out := make([]CodeInfo, 0, len(s))
	for _, v := range s {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// NewCodeSet builds a CodeSet from entries.
func NewCodeSet(entries ...CodeInfo) CodeSet {
	s := make(CodeSet, len(entries))
	for _, e := range entries {
		s[e.Code] = e
	}
	return s
}
