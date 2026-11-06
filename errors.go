package chichi

import (
	"fmt"
	"net/http"
	"strings"
)

// Kind classifies a binding error.
type Kind int

// Error kinds. Each maps to an HTTP status via Error.Status.
const (
	KindUnsupportedMediaType Kind = iota + 1 // 415
	KindBodyTooLarge                         // 413
	KindMalformed                            // 400
	KindInvalidField                         // 400
	KindValidation                           // 422
	KindInvalidTarget                        // 500: programming error
)

var kinds = map[Kind]struct {
	status int
	text   string
}{
	KindUnsupportedMediaType: {http.StatusUnsupportedMediaType, "unsupported media type"},
	KindBodyTooLarge:         {http.StatusRequestEntityTooLarge, "request body too large"},
	KindMalformed:            {http.StatusBadRequest, "malformed request"},
	KindInvalidField:         {http.StatusBadRequest, "invalid field"},
	KindValidation:           {http.StatusUnprocessableEntity, "validation failed"},
	KindInvalidTarget:        {http.StatusInternalServerError, "invalid bind target"},
}

// Sentinel errors for use with errors.Is.
var (
	ErrUnsupportedMediaType = &Error{Kind: KindUnsupportedMediaType}
	ErrBodyTooLarge         = &Error{Kind: KindBodyTooLarge}
	ErrMalformed            = &Error{Kind: KindMalformed}
	ErrInvalidField         = &Error{Kind: KindInvalidField}
	ErrValidation           = &Error{Kind: KindValidation}
	ErrInvalidTarget        = &Error{Kind: KindInvalidTarget}
)

// FieldError describes a problem with a single field.
type FieldError struct {
	Field  string `json:"field"`            // tag name, e.g. "age"
	Source string `json:"source,omitempty"` // "json", "form", "query", "path" or "" for validation
	Reason string `json:"reason"`           // e.g. "invalid integer"
}

// Error is returned by every Bind function.
//
// Error() returns a message safe to show clients. The underlying cause,
// which may contain parser internals, is kept in Err for logging.
type Error struct {
	Kind   Kind
	Fields []FieldError
	Err    error
}

// Status returns the suggested HTTP status code for the error.
func (e *Error) Status() int {
	k, ok := kinds[e.Kind]
	if !ok {
		return http.StatusInternalServerError
	}
	return k.status
}

func (e *Error) Error() string {
	msg := kinds[e.Kind].text
	if msg == "" {
		msg = "bind error"
	}
	if len(e.Fields) > 0 {
		parts := make([]string, len(e.Fields))
		for i, f := range e.Fields {
			parts[i] = fmt.Sprintf("field '%s': %s", f.Field, f.Reason)
		}
		return msg + ": " + strings.Join(parts, "; ")
	}
	if e.Kind == KindValidation && e.Err != nil {
		return msg + ": " + e.Err.Error()
	}
	return msg
}

// Unwrap returns the underlying cause.
func (e *Error) Unwrap() error { return e.Err }

// Is reports whether target is an *Error of the same Kind, so that
// errors.Is(err, chichi.ErrMalformed) works.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Kind == e.Kind
}

// ValidationError builds a validation *Error from field errors. Return it
// from Validate methods or Validator implementations.
func ValidationError(fields ...FieldError) *Error {
	return &Error{Kind: KindValidation, Fields: fields}
}

// FieldErr is shorthand for a validation error on a single field.
func FieldErr(field, reason string) *Error {
	return ValidationError(FieldError{Field: field, Reason: reason})
}

func newError(kind Kind, cause error, fields ...FieldError) *Error {
	return &Error{Kind: kind, Err: cause, Fields: fields}
}
