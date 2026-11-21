package xsd

import (
	"fmt"
	"strings"
)

// Violation is one validation failure at an XPath-like location, for
// example "/Document/FIToFICstmrCdtTrf/CdtTrfTxInf[0]/PmtId/EndToEndId".
// Indexes are zero-based, matching the Go slices.
type Violation struct {
	Path    string
	Message string
}

// Error returns "path: message".
func (v Violation) Error() string { return v.Path + ": " + v.Message }

// Violations is the error returned by the Validate methods. It lists every
// violation found, in document order, instead of stopping at the first.
type Violations []Violation

// Error lists the violations, one per line.
func (vs Violations) Error() string {
	switch len(vs) {
	case 0:
		return "no violations"
	case 1:
		return vs[0].Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d violations:", len(vs))
	for _, v := range vs {
		b.WriteString("\n\t")
		b.WriteString(v.Error())
	}
	return b.String()
}

// Unwrap returns the violations as individual errors, so that errors.As
// can extract a Violation.
func (vs Violations) Unwrap() []error {
	errs := make([]error, len(vs))
	for i, v := range vs {
		errs[i] = v
	}
	return errs
}
