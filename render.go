package chichi

import (
	"encoding/json"
	"errors"
	"net/http"
)

// JSON writes v as a JSON response with the given status. If v cannot be
// encoded, nothing is written and the error is returned.
func JSON(w http.ResponseWriter, status int, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(b)
	return err
}

// Problem is the RFC 9457 problem details body written by WriteError.
type Problem struct {
	Type   string       `json:"type"`
	Title  string       `json:"title"`
	Status int          `json:"status"`
	Detail string       `json:"detail"`
	Errors []FieldError `json:"errors,omitempty"`
}

// WriteError writes err as an application/problem+json response. An *Error
// uses its Status and client-safe message; any other error becomes a 500
// without exposing its text.
func WriteError(w http.ResponseWriter, err error) {
	p := Problem{
		Type:   "about:blank",
		Status: http.StatusInternalServerError,
		Detail: "internal server error",
	}
	var be *Error
	if errors.As(err, &be) {
		p.Status = be.Status()
		p.Detail = be.Error()
		p.Errors = be.Fields
	}
	p.Title = http.StatusText(p.Status)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
