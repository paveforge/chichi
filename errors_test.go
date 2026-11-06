package chichi_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paveforge/chichi"
)

func TestErrorStatusAndMessage(t *testing.T) {
	cause := errors.New("cause")
	tests := []struct {
		name   string
		err    *chichi.Error
		status int
		msg    string
	}{
		{"media type", chichi.ErrUnsupportedMediaType, 415, "unsupported media type"},
		{"too large", &chichi.Error{Kind: chichi.KindBodyTooLarge, Err: cause}, 413, "request body too large"},
		{"malformed hides cause", &chichi.Error{Kind: chichi.KindMalformed, Err: cause}, 400, "malformed request"},
		{
			"fields", &chichi.Error{Kind: chichi.KindInvalidField, Fields: []chichi.FieldError{
				{Field: "a", Reason: "x"}, {Field: "b", Reason: "y"},
			}}, 400, "invalid field: field 'a': x; field 'b': y",
		},
		{"validation without detail", chichi.ValidationError(), 422, "validation failed"},
		{"validation with cause", &chichi.Error{Kind: chichi.KindValidation, Err: cause}, 422, "validation failed: cause"},
		{"invalid target", chichi.ErrInvalidTarget, 500, "invalid bind target"},
		{"unknown kind", &chichi.Error{Kind: 99}, 500, "bind error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Status(); got != tt.status {
				t.Errorf("Status() = %d, want %d", got, tt.status)
			}
			if got := tt.err.Error(); got != tt.msg {
				t.Errorf("Error() = %q, want %q", got, tt.msg)
			}
		})
	}
}

func TestErrorIsAndUnwrap(t *testing.T) {
	cause := errors.New("cause")
	err := error(&chichi.Error{Kind: chichi.KindMalformed, Err: cause})
	if !errors.Is(err, chichi.ErrMalformed) {
		t.Error("should match its kind")
	}
	if errors.Is(err, chichi.ErrValidation) {
		t.Error("should not match another kind")
	}
	if !errors.Is(err, cause) {
		t.Error("should unwrap to cause")
	}
	if errors.Is(err, io.EOF) {
		t.Error("should not match unrelated error")
	}
}

func TestFieldErr(t *testing.T) {
	err := chichi.FieldErr("email", "required")
	if !errors.Is(err, chichi.ErrValidation) || err.Error() != "validation failed: field 'email': required" {
		t.Fatalf("got %v", err)
	}
}

func TestWriteError(t *testing.T) {
	t.Run("bind error", func(t *testing.T) {
		w := httptest.NewRecorder()
		chichi.WriteError(w, chichi.FieldErr("name", "required"))
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("code = %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Fatalf("content type = %q", ct)
		}
		var p chichi.Problem
		if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
			t.Fatal(err)
		}
		if p.Title != "Unprocessable Entity" || p.Status != 422 || len(p.Errors) != 1 || p.Errors[0].Field != "name" {
			t.Fatalf("got %+v", p)
		}
	})
	t.Run("other error is hidden", func(t *testing.T) {
		w := httptest.NewRecorder()
		chichi.WriteError(w, errors.New("db password leaked"))
		if w.Code != 500 || strings.Contains(w.Body.String(), "password") {
			t.Fatalf("code=%d body=%s", w.Code, w.Body)
		}
	})
}

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()
	if err := chichi.JSON(w, http.StatusCreated, map[string]int{"id": 1}); err != nil {
		t.Fatal(err)
	}
	if w.Code != 201 || w.Body.String() != `{"id":1}` || w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("code=%d ct=%q body=%s", w.Code, w.Header().Get("Content-Type"), w.Body)
	}

	w = httptest.NewRecorder()
	if err := chichi.JSON(w, http.StatusOK, make(chan int)); err == nil {
		t.Fatal("want encode error")
	}
	if w.Body.Len() != 0 || w.Header().Get("Content-Type") != "" {
		t.Fatal("nothing should be written on encode error")
	}
}
