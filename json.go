package chichi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type jsonDecoder struct {
	allowUnknown bool
}

func (d jsonDecoder) Decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	if !d.allowUnknown {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(dst); err != nil {
		return jsonError(err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return err
		}
		return newError(KindMalformed, errors.New("unexpected data after JSON value"))
	}
	return nil
}

const unknownFieldPrefix = "json: unknown field "

func jsonError(err error) error {
	var (
		mbe *http.MaxBytesError
		ute *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &mbe):
		return err
	case errors.Is(err, io.EOF):
		return newError(KindMalformed, errors.New("empty body"))
	case errors.As(err, &ute) && ute.Field != "":
		return newError(KindInvalidField, err, FieldError{
			Field: ute.Field, Source: "json", Reason: "expected " + ute.Type.String(),
		})
	case strings.HasPrefix(err.Error(), unknownFieldPrefix):
		field := strings.Trim(strings.TrimPrefix(err.Error(), unknownFieldPrefix), `"`)
		return newError(KindInvalidField, err, FieldError{
			Field: field, Source: "json", Reason: "unknown field",
		})
	}
	return newError(KindMalformed, err)
}
