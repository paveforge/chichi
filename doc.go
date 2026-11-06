// Package chichi provides request binding and response helpers for the
// chi router (github.com/go-chi/chi/v5).
//
// Bind request data into a struct with one call:
//
//	type UpdateUser struct {
//		ID    int64  `path:"id"`
//		Force bool   `query:"force"`
//		Name  string `json:"name"`
//	}
//
//	var req UpdateUser
//	if err := chichi.Bind(r, &req); err != nil {
//		chichi.WriteError(w, err)
//		return
//	}
//
// Bind picks a body decoder from the Content-Type header and also fills
// `query` and `path` tagged fields. BindJSON, BindForm, BindQuery and
// BindPath bind a single source.
//
// After binding, a configured Validator runs, followed by the target's own
// Validate method when it implements Validatable.
//
// Every failure is an *Error with a Kind that maps to an HTTP status and
// can be matched with errors.Is against ErrUnsupportedMediaType,
// ErrBodyTooLarge, ErrMalformed, ErrInvalidField, ErrValidation and
// ErrInvalidTarget.
//
// The package-level functions use Default(). Use New to build a Binder
// with other options, or pass options to a single call.
package chichi
