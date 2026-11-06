// Package structmap decodes url.Values-style maps into struct fields
// selected by a struct tag. It backs chichi's form, query and path binding.
package structmap

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ErrNotStruct is returned when the destination is not a non-nil pointer to a struct.
var ErrNotStruct = errors.New("structmap: destination must be a non-nil pointer to a struct")

// UnsupportedTypeError reports a tagged field whose type cannot be decoded.
// It indicates a programming error, not bad client input.
type UnsupportedTypeError struct {
	Field string
	Type  reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return fmt.Sprintf("structmap: field %q has unsupported type %s", e.Field, e.Type)
}

// FieldError describes one field whose value could not be decoded.
type FieldError struct {
	Field  string
	Reason string
}

// Errors collects every field that failed to decode.
type Errors []FieldError

func (e Errors) Error() string {
	parts := make([]string, len(e))
	for i, fe := range e {
		parts[i] = fmt.Sprintf("field %q: %s", fe.Field, fe.Reason)
	}
	return strings.Join(parts, "; ")
}

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

// Decode fills the struct pointed to by dst from values, using tag to map
// keys to fields. Only fields carrying tag are considered; embedded structs
// without the tag are flattened. All field errors are collected and returned
// together as Errors.
func Decode(values map[string][]string, dst any, tag string) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return ErrNotStruct
	}
	var errs Errors
	if err := decodeStruct(values, rv.Elem(), tag, &errs); err != nil {
		return err
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func decodeStruct(values map[string][]string, v reflect.Value, tag string, errs *Errors) error {
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		name, tagged := f.Tag.Lookup(tag)
		if !tagged {
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				if err := decodeStruct(values, v.Field(i), tag, errs); err != nil {
					return err
				}
			}
			continue
		}
		name, _, _ = strings.Cut(name, ",")
		if name == "-" || !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		if !supported(f.Type) {
			return &UnsupportedTypeError{Field: name, Type: f.Type}
		}
		raw := values[name]
		if len(raw) == 0 {
			continue
		}
		if err := set(v.Field(i), raw); err != nil {
			*errs = append(*errs, FieldError{Field: name, Reason: err.Error()})
		}
	}
	return nil
}

func isText(t reflect.Type) bool {
	return reflect.PointerTo(t).Implements(textUnmarshalerType)
}

func scalarSupported(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if isText(t) {
		return true
	}
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func supported(t reflect.Type) bool {
	if t.Kind() == reflect.Slice && !isText(t) {
		return scalarSupported(t.Elem())
	}
	return scalarSupported(t)
}

func set(v reflect.Value, raw []string) error {
	if v.Kind() == reflect.Slice && !isText(v.Type()) {
		s := reflect.MakeSlice(v.Type(), len(raw), len(raw))
		for i, r := range raw {
			if err := setScalar(s.Index(i), r); err != nil {
				return err
			}
		}
		v.Set(s)
		return nil
	}
	return setScalar(v, raw[0])
}

var (
	errBool  = errors.New("invalid boolean")
	errInt   = errors.New("invalid integer")
	errUint  = errors.New("invalid unsigned integer")
	errFloat = errors.New("invalid number")
	errValue = errors.New("invalid value")
)

func setScalar(v reflect.Value, s string) error {
	if v.Kind() == reflect.Pointer {
		p := reflect.New(v.Type().Elem())
		if err := setScalar(p.Elem(), s); err != nil {
			return err
		}
		v.Set(p)
		return nil
	}
	if isText(v.Type()) {
		if err := v.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s)); err != nil {
			return errValue
		}
		return nil
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return errBool
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, v.Type().Bits())
		if err != nil {
			return errInt
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, v.Type().Bits())
		if err != nil {
			return errUint
		}
		v.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(s, v.Type().Bits())
		if err != nil {
			return errFloat
		}
		v.SetFloat(n)
	}
	return nil
}
