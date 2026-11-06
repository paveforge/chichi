# chichi

[![ci](https://github.com/paveforge/chichi/actions/workflows/ci.yml/badge.svg)](https://github.com/paveforge/chichi/actions/workflows/ci.yml)

Request binding and response helpers for [chi](https://github.com/go-chi/chi).
Its only dependency is chi.

```sh
go get github.com/paveforge/chichi
```

## Binding

```go
type UpdateUser struct {
	ID    int64  `path:"id"`      // chi URL param
	Force bool   `query:"force"`  // ?force=true
	Name  string `json:"name"`    // JSON body
}

r.Put("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
	var req UpdateUser
	if err := chichi.Bind(r, &req); err != nil {
		chichi.WriteError(w, err) // problem+json with the right status
		return
	}
	chichi.JSON(w, http.StatusOK, req)
})
```

| Function    | Source                                              | Tag     |
|-------------|-----------------------------------------------------|---------|
| `Bind`      | body by Content-Type, plus query and path           | all     |
| `BindJSON`  | JSON body (`application/json`, `*+json`)            | `json`  |
| `BindForm`  | urlencoded or multipart body                        | `form`  |
| `BindQuery` | URL query                                           | `query` |
| `BindPath`  | chi URL params                                      | `path`  |

Supported field types for form, query and path: strings, bools, all int, uint
and float sizes, pointers to them, slices of them, and anything that
implements `encoding.TextUnmarshaler` (such as `time.Time` and `net.IP`).
Fields without the tag are ignored. Embedded structs are flattened. Fields that
already hold a value keep it when the key is missing, so you can set defaults
before binding.

## Configuration

Options apply to a whole `Binder` or to a single call. A per-call option wins.

```go
b := chichi.New(
	chichi.WithMaxBodyBytes(8 << 20),     // default 1 MiB; <= 0 disables
	chichi.WithMultipartMemory(16 << 20), // default 32 MiB
	chichi.WithAllowUnknownFields(),      // default: unknown JSON fields are rejected
	chichi.WithValidator(myValidator),
	chichi.WithDecoder("application/xml", xmlDecoder),
)
b.BindJSON(r, &v)

chichi.BindJSON(r, &v, chichi.WithMaxBodyBytes(4 << 10)) // one call
```

The package-level functions use `chichi.Default()`. Handlers can depend on
the `chichi.Binder` interface so you can swap or mock it.

## Validation

There's no validator by default. After binding, chichi runs:

1. the `Validator` set with `WithValidator`, if there is one, and then
2. the target's own `Validate() error` method, if it implements `chichi.Validatable`.
   `WithoutAutoValidate()` turns this step off.

```go
func (u *UpdateUser) Validate() error {
	if u.Name == "" {
		return chichi.FieldErr("name", "required")
	}
	return nil
}
```

To use a library such as go-playground/validator, wrap it in a type with a
`Validate(any) error` method.

## Errors

Every Bind function returns a `*chichi.Error`:

| Sentinel                  | Status | When                                  |
|---------------------------|--------|---------------------------------------|
| `ErrUnsupportedMediaType` | 415    | wrong Content-Type                    |
| `ErrBodyTooLarge`         | 413    | body exceeds the limit                |
| `ErrMalformed`            | 400    | syntax error, empty body, trailing data |
| `ErrInvalidField`         | 400    | type mismatch or unknown field (`Fields` set) |
| `ErrValidation`           | 422    | the validator or `Validate()` failed  |
| `ErrInvalidTarget`        | 500    | programming error (bad `dst`)         |

```go
errors.Is(err, chichi.ErrBodyTooLarge)

var be *chichi.Error
if errors.As(err, &be) { log.Println(be.Status(), be.Fields, be.Err) }
```

`Error()` is safe to show clients. The raw parser error is kept in `Err` and
is never written to the response. `WriteError` writes RFC 9457 problem details,
and any error that isn't a chichi error becomes an opaque 500.

## Development

```sh
make test   # tests with -race
make cover  # fails below 100% coverage
make lint   # golangci-lint
make fuzz
```

CI enforces 100% statement coverage, `go vet`, golangci-lint and a short fuzz run.

## License

[MIT](LICENSE)
