package chichi

import (
	"errors"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/paveforge/chichi/internal/structmap"
)

// Binder binds request data into Go values.
type Binder interface {
	// Bind decodes the body according to its Content-Type (if any) and
	// fills `query` and `path` tagged fields.
	Bind(r *http.Request, dst any, opts ...Option) error
	// BindJSON decodes a JSON body.
	BindJSON(r *http.Request, dst any, opts ...Option) error
	// BindForm decodes an urlencoded or multipart form body into `form` tagged fields.
	BindForm(r *http.Request, dst any, opts ...Option) error
	// BindQuery decodes the URL query into `query` tagged fields.
	BindQuery(r *http.Request, dst any, opts ...Option) error
	// BindPath decodes chi URL parameters into `path` tagged fields.
	BindPath(r *http.Request, dst any, opts ...Option) error
}

type binder struct {
	cfg config
}

var _ Binder = (*binder)(nil)

// New returns a Binder configured with opts.
func New(opts ...Option) Binder {
	b := &binder{cfg: defaultConfig()}
	for _, o := range opts {
		o(&b.cfg)
	}
	return b
}

var std = New()

// Default returns the Binder used by the package-level functions.
func Default() Binder { return std }

// Bind calls Default().Bind.
func Bind(r *http.Request, dst any, opts ...Option) error { return std.Bind(r, dst, opts...) }

// BindJSON calls Default().BindJSON.
func BindJSON(r *http.Request, dst any, opts ...Option) error { return std.BindJSON(r, dst, opts...) }

// BindForm calls Default().BindForm.
func BindForm(r *http.Request, dst any, opts ...Option) error { return std.BindForm(r, dst, opts...) }

// BindQuery calls Default().BindQuery.
func BindQuery(r *http.Request, dst any, opts ...Option) error { return std.BindQuery(r, dst, opts...) }

// BindPath calls Default().BindPath.
func BindPath(r *http.Request, dst any, opts ...Option) error { return std.BindPath(r, dst, opts...) }

func (b *binder) with(opts []Option) *config {
	c := b.cfg
	for _, o := range opts {
		o(&c)
	}
	return &c
}

const (
	mimeJSON      = "application/json"
	mimeForm      = "application/x-www-form-urlencoded"
	mimeMultipart = "multipart/form-data"
)

func (b *binder) Bind(r *http.Request, dst any, opts ...Option) error {
	c := b.with(opts)
	if err := checkTarget(dst); err != nil {
		return err
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && r.ContentLength != 0 {
		d, err := c.decoderFor(mediaType(ct))
		if err != nil {
			return err
		}
		if err := c.decodeBody(r, dst, d); err != nil {
			return err
		}
	}
	if err := decodeValues(r.URL.Query(), dst, "query"); err != nil {
		return err
	}
	if err := decodeValues(pathValues(r), dst, "path"); err != nil {
		return err
	}
	return c.validate(dst)
}

func (b *binder) BindJSON(r *http.Request, dst any, opts ...Option) error {
	c := b.with(opts)
	if err := checkTarget(dst); err != nil {
		return err
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && !isJSON(mediaType(ct)) {
		return newError(KindUnsupportedMediaType, nil)
	}
	if err := c.decodeBody(r, dst, jsonDecoder{allowUnknown: c.allowUnknown}); err != nil {
		return err
	}
	return c.validate(dst)
}

func (b *binder) BindForm(r *http.Request, dst any, opts ...Option) error {
	c := b.with(opts)
	if err := checkTarget(dst); err != nil {
		return err
	}
	if mt := mediaType(r.Header.Get("Content-Type")); mt != mimeForm && mt != mimeMultipart {
		return newError(KindUnsupportedMediaType, nil)
	}
	if err := c.decodeBody(r, dst, formDecoder{maxMemory: c.multipartMemory}); err != nil {
		return err
	}
	return c.validate(dst)
}

func (b *binder) BindQuery(r *http.Request, dst any, opts ...Option) error {
	return b.bindValues(r.URL.Query(), dst, "query", opts)
}

func (b *binder) BindPath(r *http.Request, dst any, opts ...Option) error {
	return b.bindValues(pathValues(r), dst, "path", opts)
}

func (b *binder) bindValues(values url.Values, dst any, source string, opts []Option) error {
	c := b.with(opts)
	if err := checkTarget(dst); err != nil {
		return err
	}
	if err := decodeValues(values, dst, source); err != nil {
		return err
	}
	return c.validate(dst)
}

func (c *config) decoderFor(mt string) (Decoder, error) {
	if d, ok := c.decoders[mt]; ok {
		return d, nil
	}
	switch {
	case isJSON(mt):
		return jsonDecoder{allowUnknown: c.allowUnknown}, nil
	case mt == mimeForm || mt == mimeMultipart:
		return formDecoder{maxMemory: c.multipartMemory}, nil
	}
	return nil, newError(KindUnsupportedMediaType, nil)
}

// decodeBody applies the body limit, runs d and normalizes its error.
func (c *config) decodeBody(r *http.Request, dst any, d Decoder) error {
	if r.Body == nil {
		r.Body = http.NoBody
	}
	if c.maxBodyBytes > 0 {
		r.Body = http.MaxBytesReader(nil, r.Body, c.maxBodyBytes)
	}
	err := d.Decode(r, dst)
	if err == nil {
		return nil
	}
	var be *Error
	if errors.As(err, &be) {
		return be
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return newError(KindBodyTooLarge, err)
	}
	return newError(KindMalformed, err)
}

func (c *config) validate(dst any) error {
	if c.validator != nil {
		if err := c.validator.Validate(dst); err != nil {
			return validationError(err)
		}
	}
	if v, ok := dst.(Validatable); ok && c.autoValidate {
		if err := v.Validate(); err != nil {
			return validationError(err)
		}
	}
	return nil
}

func validationError(err error) error {
	var be *Error
	if errors.As(err, &be) && be.Kind == KindValidation {
		return be
	}
	return newError(KindValidation, err)
}

func checkTarget(dst any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return newError(KindInvalidTarget, errors.New("chichi: destination must be a non-nil pointer"))
	}
	return nil
}

func decodeValues(values url.Values, dst any, source string) error {
	err := structmap.Decode(values, dst, source)
	if err == nil {
		return nil
	}
	var fes structmap.Errors
	if errors.As(err, &fes) {
		fields := make([]FieldError, len(fes))
		for i, fe := range fes {
			fields[i] = FieldError{Field: fe.Field, Source: source, Reason: fe.Reason}
		}
		return newError(KindInvalidField, err, fields...)
	}
	return newError(KindInvalidTarget, err)
}

func pathValues(r *http.Request) url.Values {
	values := url.Values{}
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		for i, k := range rctx.URLParams.Keys {
			values.Add(k, rctx.URLParams.Values[i])
		}
	}
	return values
}

func mediaType(ct string) string {
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return ""
	}
	return mt
}

func isJSON(mt string) bool {
	return mt == mimeJSON || strings.HasSuffix(mt, "+json")
}
