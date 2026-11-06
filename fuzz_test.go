package chichi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paveforge/chichi"
)

type fuzzTarget struct {
	S  string   `json:"s" query:"s"`
	I  int      `json:"i" query:"i"`
	U  uint8    `json:"u" query:"u"`
	F  float64  `json:"f" query:"f"`
	B  bool     `json:"b" query:"b"`
	P  *int     `json:"p" query:"p"`
	SS []string `json:"ss" query:"ss"`
}

// Every failure must be a *chichi.Error with a 4xx status; nothing may panic.
func checkFuzzErr(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	var be *chichi.Error
	if !errors.As(err, &be) {
		t.Fatalf("non-chichi error: %T %v", err, err)
	}
	if be.Status() < 400 || be.Status() >= 500 {
		t.Fatalf("unexpected status %d for %v", be.Status(), err)
	}
}

func FuzzBindJSON(f *testing.F) {
	for _, s := range []string{``, `{}`, `{"s":"x","i":1,"u":2,"f":1.5,"b":true,"p":3,"ss":["a"]}`, `{"i":"x"}`, `[`, `{} {}`, `null`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		checkFuzzErr(t, chichi.BindJSON(r, &fuzzTarget{}, chichi.WithMaxBodyBytes(4096)))
	})
}

func FuzzBindQuery(f *testing.F) {
	for _, s := range []string{``, `s=x&i=1&u=2&f=1.5&b=true&p=3&ss=a&ss=b`, `i=x`, `u=300`, `%zz`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, query string) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.URL.RawQuery = query
		checkFuzzErr(t, chichi.BindQuery(r, &fuzzTarget{}))
	})
}
