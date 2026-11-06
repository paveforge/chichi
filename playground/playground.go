// Package playground adapts github.com/go-playground/validator/v10 to
// chichi.Validator.
//
// It is a separate Go module, so chichi itself does not depend on the
// validator library:
//
//	binder := chichi.New(chichi.WithValidator(playground.NewValidator()))
package playground

import (
	"errors"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/paveforge/chichi"
)

// Validator implements chichi.Validator with go-playground/validator.
type Validator struct {
	v *validator.Validate
}

var _ chichi.Validator = (*Validator)(nil)

// NewValidator returns a Validator backed by a new validator.Validate with
// WithRequiredStructEnabled plus any opts. Field names in errors come from
// the json, form, query or path tag ("email" rather than "Email").
func NewValidator(opts ...validator.Option) *Validator {
	opts = append([]validator.Option{validator.WithRequiredStructEnabled()}, opts...)
	v := validator.New(opts...)
	v.RegisterTagNameFunc(TagName)
	return &Validator{v: v}
}

// Wrap returns a Validator that uses v as is, e.g. one with custom rules
// already registered. Register TagName on v to report tag names in errors.
func Wrap(v *validator.Validate) *Validator {
	return &Validator{v: v}
}

// Engine returns the underlying validator.Validate, for registering custom
// validations, aliases or translations.
func (p *Validator) Engine() *validator.Validate { return p.v }

// TagName returns the field's json, form, query or path tag name, falling
// back to the Go field name. Use it with validator.RegisterTagNameFunc.
func TagName(f reflect.StructField) string {
	for _, tag := range []string{"json", "form", "query", "path"} {
		if name, _, _ := strings.Cut(f.Tag.Get(tag), ","); name != "" && name != "-" {
			return name
		}
	}
	return f.Name
}

// Validate validates structs (or pointers to structs) and turns failures
// into a chichi validation error (HTTP 422). Values of other kinds, such as
// maps or slices, are not validated and return nil.
func (p *Validator) Validate(v any) error {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}

	err := p.v.Struct(v)
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		fields := make([]chichi.FieldError, len(verrs))
		for i, fe := range verrs {
			reason := fe.Tag() // e.g. "required", "email", "gte"
			if fe.Param() != "" {
				reason += "=" + fe.Param() // e.g. "gte=18"
			}
			// Namespace is "CreateUser.address.city"; drop the root struct name.
			_, field, _ := strings.Cut(fe.Namespace(), ".")
			fields[i] = chichi.FieldError{Field: field, Reason: reason}
		}
		return chichi.ValidationError(fields...)
	}
	return err // nil, or an unexpected error from the library
}
