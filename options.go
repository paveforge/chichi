package chichi

import "net/http"

// Defaults used by New.
const (
	DefaultMaxBodyBytes    int64 = 1 << 20  // 1 MiB
	DefaultMultipartMemory int64 = 32 << 20 // 32 MiB
)

// Decoder decodes a request body into dst. Register custom decoders for
// additional media types with WithDecoder.
type Decoder interface {
	Decode(r *http.Request, dst any) error
}

// DecoderFunc adapts a function to Decoder.
type DecoderFunc func(r *http.Request, dst any) error

// Decode calls f(r, dst).
func (f DecoderFunc) Decode(r *http.Request, dst any) error { return f(r, dst) }

// Validator validates a bound value, e.g. an adapter for a validation library.
type Validator interface {
	Validate(v any) error
}

// Validatable is implemented by types that validate themselves. Bind
// functions call Validate after a successful bind unless disabled with
// WithoutAutoValidate.
type Validatable interface {
	Validate() error
}

// Option configures a Binder (via New) or a single Bind call.
type Option func(*config)

type config struct {
	maxBodyBytes    int64
	multipartMemory int64
	allowUnknown    bool
	autoValidate    bool
	validator       Validator
	decoders        map[string]Decoder
}

func defaultConfig() config {
	return config{
		maxBodyBytes:    DefaultMaxBodyBytes,
		multipartMemory: DefaultMultipartMemory,
		autoValidate:    true,
	}
}

// WithMaxBodyBytes limits the request body size. n <= 0 disables the limit.
func WithMaxBodyBytes(n int64) Option {
	return func(c *config) { c.maxBodyBytes = n }
}

// WithMultipartMemory sets how much of a multipart body is kept in memory;
// the rest of file parts spill to disk.
func WithMultipartMemory(n int64) Option {
	return func(c *config) { c.multipartMemory = n }
}

// WithAllowUnknownFields accepts JSON fields that do not exist in the target.
// By default they are rejected.
func WithAllowUnknownFields() Option {
	return func(c *config) { c.allowUnknown = true }
}

// WithValidator runs v after every successful bind.
func WithValidator(v Validator) Option {
	return func(c *config) { c.validator = v }
}

// WithoutAutoValidate stops Bind functions from calling Validatable.Validate.
func WithoutAutoValidate() Option {
	return func(c *config) { c.autoValidate = false }
}

// WithDecoder registers d for mediaType (e.g. "application/xml") in Bind.
// It takes precedence over the built-in decoders.
func WithDecoder(mediaType string, d Decoder) Option {
	return func(c *config) {
		m := make(map[string]Decoder, len(c.decoders)+1)
		for k, v := range c.decoders {
			m[k] = v
		}
		m[mediaType] = d
		c.decoders = m
	}
}
