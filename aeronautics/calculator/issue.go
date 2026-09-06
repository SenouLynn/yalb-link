package calculator

import (
	"errors"
)

// IssueKind classifies why a calculation could not return a usable result.
// The kinds are reported separately from numeric validity so a caller can tell
// "you have not told me yet" from "this cannot be true" from "this model does
// not cover your design".
type IssueKind uint8

const (
	// IssueMissing means a required input was not supplied.
	IssueMissing IssueKind = iota
	// IssueInvalid means a supplied input cannot be physically valid, such as a
	// negative area or a zero denominator.
	IssueInvalid
	// IssueUnsupported means the input is well formed but outside what the
	// implemented model may claim, such as a 2D section coefficient offered as a
	// whole-aircraft CLmax.
	IssueUnsupported
)

var issueKindNames = [...]string{
	IssueMissing:     "missing",
	IssueInvalid:     "invalid",
	IssueUnsupported: "unsupported",
}

// String returns the issue kind's lowercase name.
func (k IssueKind) String() string {
	if int(k) < len(issueKindNames) {
		return issueKindNames[k]
	}
	return "unknown"
}

// Issue is a field-specific reason a calculation did not produce a result.
type Issue struct {
	// Field names the input the issue belongs to, using the equation's port name.
	Field string
	// Detail explains the problem in terms the worksheet can show a builder.
	Detail string
	// Kind separates missing, invalid and unsupported input.
	Kind IssueKind
}

// Error renders the issue as "field: kind: detail".
func (i Issue) Error() string {
	return i.Field + ": " + i.Kind.String() + ": " + i.Detail
}

// Issues is a set of field-specific issues from one evaluation. It implements
// error so an evaluation can return every problem at once rather than only the
// first, which is what lets a worksheet annotate several fields in one pass.
type Issues []Issue

// Error renders every issue, separated by "; ". The joining is written out
// rather than using strings.Join: strings is only reachable through the
// boundary allowlist's "testing" entry, and the production core does not lean
// on that.
func (is Issues) Error() string {
	message := ""
	for n, issue := range is {
		if n > 0 {
			message += "; "
		}
		message += issue.Error()
	}
	return message
}

// Kind reports whether any issue in the set has the given kind.
func (is Issues) Kind(k IssueKind) bool {
	for _, issue := range is {
		if issue.Kind == k {
			return true
		}
	}
	return false
}

// Fields returns the names of the fields carrying issues, in the order the
// evaluation recorded them.
func (is Issues) Fields() []string {
	fields := make([]string, 0, len(is))
	for _, issue := range is {
		fields = append(fields, issue.Field)
	}
	return fields
}

// AsIssues extracts the Issues carried by err, if any.
func AsIssues(err error) (Issues, bool) {
	var is Issues
	if errors.As(err, &is) {
		return is, true
	}
	return nil, false
}
