package chichi_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

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
	if c.Age < 18 {
		return chichi.FieldErr("age", "must be at least 18")
	}
	return nil
}

func Example() {
	r := chi.NewRouter()
	r.Post("/orgs/{org}/users", func(w http.ResponseWriter, r *http.Request) {
		var req CreateUser
		if err := chichi.Bind(r, &req); err != nil {
			chichi.WriteError(w, err)
			return
		}
		_ = chichi.JSON(w, http.StatusCreated, req)
	})

	for _, body := range []string{`{"email":"a@b.c","age":30}`, `{"email":"a@b.c","age":3}`} {
		req := httptest.NewRequest(http.MethodPost, "/orgs/7/users?dry_run=true", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		fmt.Println(w.Code, strings.TrimSpace(w.Body.String()))
	}
	// Output:
	// 201 {"OrgID":7,"DryRun":true,"email":"a@b.c","age":30}
	// 422 {"type":"about:blank","title":"Unprocessable Entity","status":422,"detail":"validation failed: field 'age': must be at least 18","errors":[{"field":"age","reason":"must be at least 18"}]}
}

func ExampleBindQuery() {
	type ListParams struct {
		Page int      `query:"page"`
		Tags []string `query:"tag"`
	}
	r := httptest.NewRequest(http.MethodGet, "/items?page=2&tag=a&tag=b", nil)

	p := ListParams{Page: 1} // defaults survive when a key is absent
	if err := chichi.BindQuery(r, &p); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(p.Page, p.Tags)
	// Output: 2 [a b]
}

func ExampleNew() {
	b := chichi.New(chichi.WithMaxBodyBytes(16))
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"too-long@example.com"}`))

	err := b.BindJSON(r, &CreateUser{})
	var be *chichi.Error
	if errors.As(err, &be) {
		fmt.Println(be.Status(), be)
	}
	// Output: 413 request body too large
}
