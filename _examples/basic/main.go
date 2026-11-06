// Command basic is a small chi server showing chichi in use.
//
//	go run ./_examples/basic
//	curl -X POST 'localhost:8080/orgs/1/users?dry_run=true' \
//	  -H 'Content-Type: application/json' -d '{"email":"a@b.c","age":30}'
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/paveforge/chichi"
)

type CreateUser struct {
	OrgID  int64  `path:"org"`
	DryRun bool   `query:"dry_run"`
	Email  string `json:"email"`
	Age    int    `json:"age"`
}

func (c *CreateUser) Validate() error {
	var fields []chichi.FieldError
	if c.Email == "" {
		fields = append(fields, chichi.FieldError{Field: "email", Reason: "required"})
	}
	if c.Age < 18 {
		fields = append(fields, chichi.FieldError{Field: "age", Reason: "must be at least 18"})
	}
	if len(fields) > 0 {
		return chichi.ValidationError(fields...)
	}
	return nil
}

func main() {
	binder := chichi.New(chichi.WithMaxBodyBytes(64 << 10))

	r := chi.NewRouter()
	r.Post("/orgs/{org}/users", func(w http.ResponseWriter, r *http.Request) {
		var req CreateUser
		if err := binder.Bind(r, &req); err != nil {
			chichi.WriteError(w, err)
			return
		}
		_ = chichi.JSON(w, http.StatusCreated, req)
	})

	srv := &http.Server{Addr: ":8080", Handler: r, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
