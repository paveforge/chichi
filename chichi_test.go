package chichi_test

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/paveforge/chichi"
)

// ---- fixtures ----

type user struct {
	ID    int64  `path:"id"`
	Force bool   `query:"force"`
	Name  string `json:"name" form:"name"`
	Age   int    `json:"age" form:"age"`
}

type validated struct {
	Name string `json:"name" query:"name" path:"name" form:"name"`
}

func (v *validated) Validate() error {
	if v.Name == "" {
		return chichi.FieldErr("name", "required")
	}
	return nil
}

type badQuery struct {
	M map[string]string `query:"m"`
}

type badPath struct {
	M map[string]string `path:"m"`
}

type validatorFunc func(any) error

func (f validatorFunc) Validate(v any) error { return f(v) }

func jsonReq(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func formReq(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func multipartReq(t *testing.T, fields map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

// withPath attaches chi URL params to r, as chi does when routing.
func withPath(r *http.Request, kv ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i < len(kv); i += 2 {
		rctx.URLParams.Add(kv[i], kv[i+1])
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func wantKind(t *testing.T, err, kind error) {
	t.Helper()
	_ = bindErr(t, err, kind)
}

func bindErr(t *testing.T, err, kind error) *chichi.Error {
	t.Helper()
	if !errors.Is(err, kind) {
		t.Fatalf("got %v, want %v", err, kind)
	}
	var be *chichi.Error
	if !errors.As(err, &be) {
		t.Fatalf("got %T, want *chichi.Error", err)
	}
	return be
}

// ---- Bind (auto) ----

func TestBindCombinesSources(t *testing.T) {
	r := chi.NewRouter()
	var got user
	var bindErr error
	r.Put("/users/{id}", func(_ http.ResponseWriter, r *http.Request) {
		bindErr = chichi.Bind(r, &got)
	})
	req := httptest.NewRequest(http.MethodPut, "/users/42?force=true", strings.NewReader(`{"name":"ann","age":30}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if bindErr != nil {
		t.Fatal(bindErr)
	}
	if want := (user{ID: 42, Force: true, Name: "ann", Age: 30}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestBindForm(t *testing.T) {
	var got user
	if err := chichi.Bind(formReq("name=bo&age=5"), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "bo" || got.Age != 5 {
		t.Fatalf("got %+v", got)
	}
}

func TestBindSkipsBodyWithoutContentTypeOrLength(t *testing.T) {
	var got user
	r := httptest.NewRequest(http.MethodGet, "/?force=1", nil)
	r.Header.Set("Content-Type", "application/json") // but ContentLength == 0
	if err := chichi.Bind(r, &got); err != nil || !got.Force {
		t.Fatalf("got %+v, %v", got, err)
	}
	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ignored"))
	if err := chichi.Bind(r, &got); err != nil {
		t.Fatal(err)
	}
}

func TestBindErrors(t *testing.T) {
	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	xml := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("<a/>"))
	xml.Header.Set("Content-Type", "application/xml")

	tests := []struct {
		name string
		r    *http.Request
		dst  any
		kind error
	}{
		{"invalid target", plain, user{}, chichi.ErrInvalidTarget},
		{"unsupported media type", xml, &user{}, chichi.ErrUnsupportedMediaType},
		{"bad body", jsonReq(`{`), &user{}, chichi.ErrMalformed},
		{"bad query", httptest.NewRequest(http.MethodGet, "/?force=x", nil), &user{}, chichi.ErrInvalidField},
		{"bad path", withPath(plain, "id", "x"), &user{}, chichi.ErrInvalidField},
		{"validation", plain, &validated{}, chichi.ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantKind(t, chichi.Bind(tt.r, tt.dst), tt.kind)
		})
	}
}

func TestBindCustomDecoder(t *testing.T) {
	called := false
	dec := chichi.DecoderFunc(func(_ *http.Request, dst any) error {
		called = true
		dst.(*validated).Name = "from-xml"
		return nil
	})
	b := chichi.New(chichi.WithDecoder("application/xml", dec), chichi.WithDecoder("text/plain", dec))
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("<a/>"))
	r.Header.Set("Content-Type", "application/xml")
	var got validated
	if err := b.Bind(r, &got); err != nil || !called || got.Name != "from-xml" {
		t.Fatalf("got %+v, called=%v, err=%v", got, called, err)
	}
}

func TestCustomDecoderErrors(t *testing.T) {
	plainErr := errors.New("boom")
	dec := chichi.DecoderFunc(func(*http.Request, any) error { return plainErr })
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x"))
	r.Header.Set("Content-Type", "text/csv")
	be := bindErr(t, chichi.Bind(r, &user{}, chichi.WithDecoder("text/csv", dec)), chichi.ErrMalformed)
	if !errors.Is(be, plainErr) {
		t.Fatal("cause not preserved")
	}
}

// ---- BindJSON ----

func TestBindJSON(t *testing.T) {
	var got user
	if err := chichi.BindJSON(jsonReq(`{"name":"ann","age":3}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "ann" || got.Age != 3 {
		t.Fatalf("got %+v", got)
	}

	// A missing Content-Type is accepted, and so are +json suffixes.
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"x"}`))
	if err := chichi.BindJSON(r, &got); err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"x"}`))
	r.Header.Set("Content-Type", "application/merge-patch+json")
	if err := chichi.BindJSON(r, &got); err != nil {
		t.Fatal(err)
	}

	// Non-struct targets work for JSON.
	var m map[string]int
	if err := chichi.BindJSON(jsonReq(`{"a":1}`), &m); err != nil || m["a"] != 1 {
		t.Fatalf("got %v, %v", m, err)
	}
}

func TestBindJSONErrors(t *testing.T) {
	wrongCT := jsonReq(`{}`)
	wrongCT.Header.Set("Content-Type", "text/plain")
	big := jsonReq(`{"name":"` + strings.Repeat("a", 100) + `"}`)
	trailingBig := jsonReq(`{}` + strings.Repeat(" ", 2000) + `x`)

	tests := []struct {
		name   string
		r      *http.Request
		dst    any
		opts   []chichi.Option
		kind   error
		fields []chichi.FieldError
	}{
		{name: "nil target", r: jsonReq(`{}`), dst: (*user)(nil), kind: chichi.ErrInvalidTarget},
		{name: "wrong content type", r: wrongCT, dst: &user{}, kind: chichi.ErrUnsupportedMediaType},
		{name: "empty body", r: jsonReq(``), dst: &user{}, kind: chichi.ErrMalformed},
		{name: "syntax", r: jsonReq(`{"name":`), dst: &user{}, kind: chichi.ErrMalformed},
		{name: "top-level type", r: jsonReq(`[]`), dst: &user{}, kind: chichi.ErrMalformed},
		{name: "trailing data", r: jsonReq(`{} {}`), dst: &user{}, kind: chichi.ErrMalformed},
		{name: "too large", r: big, dst: &user{}, opts: []chichi.Option{chichi.WithMaxBodyBytes(10)}, kind: chichi.ErrBodyTooLarge},
		{name: "too large after value", r: trailingBig, dst: &user{}, opts: []chichi.Option{chichi.WithMaxBodyBytes(1000)}, kind: chichi.ErrBodyTooLarge},
		{
			name: "field type", r: jsonReq(`{"age":"x"}`), dst: &user{}, kind: chichi.ErrInvalidField,
			fields: []chichi.FieldError{{Field: "age", Source: "json", Reason: "expected int"}},
		},
		{
			name: "unknown field", r: jsonReq(`{"nope":1}`), dst: &user{}, kind: chichi.ErrInvalidField,
			fields: []chichi.FieldError{{Field: "nope", Source: "json", Reason: "unknown field"}},
		},
		{name: "validation", r: jsonReq(`{}`), dst: &validated{}, kind: chichi.ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be := bindErr(t, chichi.BindJSON(tt.r, tt.dst, tt.opts...), tt.kind)
			if tt.fields != nil && !reflect.DeepEqual(be.Fields, tt.fields) {
				t.Fatalf("fields = %+v, want %+v", be.Fields, tt.fields)
			}
		})
	}
}

func TestBindJSONOptions(t *testing.T) {
	var got user
	if err := chichi.BindJSON(jsonReq(`{"nope":1}`), &got, chichi.WithAllowUnknownFields()); err != nil {
		t.Fatal(err)
	}
	big := jsonReq(`{"name":"` + strings.Repeat("a", 2<<20) + `"}`)
	if err := chichi.BindJSON(big, &got, chichi.WithMaxBodyBytes(0)); err != nil {
		t.Fatalf("limit disabled: %v", err)
	}
	r := &http.Request{Header: http.Header{}} // nil Body
	wantKind(t, chichi.BindJSON(r, &got), chichi.ErrMalformed)
}

// ---- BindForm ----

func TestBindFormDirect(t *testing.T) {
	var got user
	if err := chichi.BindForm(formReq("name=ann&age=7"), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "ann" || got.Age != 7 {
		t.Fatalf("got %+v", got)
	}

	got = user{}
	r := multipartReq(t, map[string]string{"name": "mp", "age": "8"})
	if err := chichi.BindForm(r, &got, chichi.WithMultipartMemory(1<<10)); err != nil {
		t.Fatal(err)
	}
	if got.Name != "mp" || got.Age != 8 {
		t.Fatalf("got %+v", got)
	}
}

func TestBindFormErrors(t *testing.T) {
	badMultipart := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("garbage"))
	badMultipart.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	badCT := formReq("")
	badCT.Header.Set("Content-Type", "multipart/form-data; ===")

	tests := []struct {
		name string
		r    *http.Request
		dst  any
		opts []chichi.Option
		kind error
	}{
		{name: "invalid target", r: formReq(""), dst: 1, kind: chichi.ErrInvalidTarget},
		{name: "json content type", r: jsonReq(`{}`), dst: &user{}, kind: chichi.ErrUnsupportedMediaType},
		{name: "unparsable content type", r: badCT, dst: &user{}, kind: chichi.ErrUnsupportedMediaType},
		{name: "bad urlencoding", r: formReq("name=%zz"), dst: &user{}, kind: chichi.ErrMalformed},
		{name: "bad multipart", r: badMultipart, dst: &user{}, kind: chichi.ErrMalformed},
		{name: "too large", r: formReq("name=" + strings.Repeat("a", 100)), dst: &user{}, opts: []chichi.Option{chichi.WithMaxBodyBytes(10)}, kind: chichi.ErrBodyTooLarge},
		{name: "bad field", r: formReq("age=x"), dst: &user{}, kind: chichi.ErrInvalidField},
		{name: "validation", r: formReq(""), dst: &validated{}, kind: chichi.ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantKind(t, chichi.BindForm(tt.r, tt.dst, tt.opts...), tt.kind)
		})
	}
}

// ---- BindQuery / BindPath ----

func TestBindQuery(t *testing.T) {
	var got user
	if err := chichi.BindQuery(httptest.NewRequest(http.MethodGet, "/?force=true", nil), &got); err != nil || !got.Force {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestBindPath(t *testing.T) {
	var got user
	r := withPath(httptest.NewRequest(http.MethodGet, "/", nil), "id", "9")
	if err := chichi.BindPath(r, &got); err != nil || got.ID != 9 {
		t.Fatalf("got %+v, %v", got, err)
	}
	// No chi route context: nothing to bind.
	got = user{}
	if err := chichi.BindPath(httptest.NewRequest(http.MethodGet, "/", nil), &got); err != nil || got.ID != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestBindValuesErrors(t *testing.T) {
	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	n := 1
	tests := []struct {
		name string
		bind func(*http.Request, any, ...chichi.Option) error
		r    *http.Request
		dst  any
		kind error
	}{
		{"query nil target", chichi.BindQuery, plain, nil, chichi.ErrInvalidTarget},
		{"query non-struct", chichi.BindQuery, plain, &n, chichi.ErrInvalidTarget},
		{"query unsupported field", chichi.BindQuery, plain, &badQuery{}, chichi.ErrInvalidTarget},
		{"query bad value", chichi.BindQuery, httptest.NewRequest(http.MethodGet, "/?force=x", nil), &user{}, chichi.ErrInvalidField},
		{"query validation", chichi.BindQuery, plain, &validated{}, chichi.ErrValidation},
		{"path unsupported field", chichi.BindPath, plain, &badPath{}, chichi.ErrInvalidTarget},
		{"path bad value", chichi.BindPath, withPath(plain, "id", "x"), &user{}, chichi.ErrInvalidField},
		{"path validation", chichi.BindPath, plain, &validated{}, chichi.ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantKind(t, tt.bind(tt.r, tt.dst), tt.kind)
		})
	}
}

// ---- validation ----

func TestValidation(t *testing.T) {
	plain := httptest.NewRequest(http.MethodGet, "/?name=ok", nil)
	empty := httptest.NewRequest(http.MethodGet, "/", nil)
	plainErr := errors.New("nope")

	t.Run("validator error is wrapped", func(t *testing.T) {
		b := chichi.New(chichi.WithValidator(validatorFunc(func(any) error { return plainErr })))
		be := bindErr(t, b.BindQuery(plain, &validated{}), chichi.ErrValidation)
		if !errors.Is(be, plainErr) || be.Error() != "validation failed: nope" {
			t.Fatalf("got %v", be)
		}
	})
	t.Run("validator *Error of another kind is wrapped", func(t *testing.T) {
		b := chichi.New(chichi.WithValidator(validatorFunc(func(any) error { return chichi.ErrMalformed })))
		wantKind(t, b.BindQuery(plain, &validated{}), chichi.ErrValidation)
	})
	t.Run("validator passes, Validatable runs", func(t *testing.T) {
		b := chichi.New(chichi.WithValidator(validatorFunc(func(any) error { return nil })))
		be := bindErr(t, b.BindQuery(empty, &validated{}), chichi.ErrValidation)
		want := []chichi.FieldError{{Field: "name", Reason: "required"}}
		if !reflect.DeepEqual(be.Fields, want) {
			t.Fatalf("fields = %+v", be.Fields)
		}
	})
	t.Run("auto validate disabled", func(t *testing.T) {
		if err := chichi.BindQuery(empty, &validated{}, chichi.WithoutAutoValidate()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("passes", func(t *testing.T) {
		if err := chichi.BindQuery(plain, &validated{}); err != nil {
			t.Fatal(err)
		}
	})
}

// ---- Binder / Default ----

func TestDefaultAndNew(t *testing.T) {
	if chichi.Default() == nil {
		t.Fatal("nil Default")
	}
	b := chichi.New(chichi.WithMaxBodyBytes(5))
	wantKind(t, b.BindJSON(jsonReq(`{"name":"long"}`), &user{}), chichi.ErrBodyTooLarge)
	// Per-call options override the Binder's without mutating it.
	if err := b.BindJSON(jsonReq(`{"name":"long"}`), &user{}, chichi.WithMaxBodyBytes(100)); err != nil {
		t.Fatal(err)
	}
	wantKind(t, b.BindJSON(jsonReq(`{"name":"long"}`), &user{}), chichi.ErrBodyTooLarge)
}
