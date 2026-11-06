package playground_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/paveforge/chichi"
	"github.com/paveforge/chichi/playground"
)

type address struct {
	City string `json:"city" validate:"required"`
}

type createUser struct {
	Email   string   `json:"email" validate:"required,email"`
	Age     int      `json:"age" validate:"gte=18"`
	Nick    string   `query:"nick" validate:"omitempty,min=3"`
	Plain   string   `validate:"omitempty,max=2"`
	Hidden  string   `json:"-" form:"hidden_name" validate:"omitempty,max=1"`
	Address *address `json:"address" validate:"required"`
}

func fields(t *testing.T, err error) []chichi.FieldError {
	t.Helper()
	var e *chichi.Error
	if !errors.As(err, &e) {
		t.Fatalf("want *chichi.Error, got %T %v", err, err)
	}
	if !errors.Is(err, chichi.ErrValidation) || e.Status() != http.StatusUnprocessableEntity {
		t.Fatalf("want validation error, got %v", err)
	}
	return e.Fields
}

func TestValidate(t *testing.T) {
	v := playground.NewValidator()
	got := fields(t, v.Validate(&createUser{
		Email: "nope", Age: 10, Nick: "ab", Plain: "abc", Hidden: "xy",
		Address: &address{},
	}))
	want := []chichi.FieldError{
		{Field: "email", Reason: "email"},
		{Field: "age", Reason: "gte=18"},
		{Field: "nick", Reason: "min=3"},
		{Field: "Plain", Reason: "max=2"},
		{Field: "hidden_name", Reason: "max=1"},
		{Field: "address.city", Reason: "required"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestValidateOK(t *testing.T) {
	v := playground.NewValidator()
	ok := createUser{Email: "a@b.co", Age: 20, Address: &address{City: "x"}}
	if err := v.Validate(&ok); err != nil {
		t.Fatal(err)
	}
	if err := v.Validate(ok); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNonStruct(t *testing.T) {
	v := playground.NewValidator()
	var nilUser *createUser
	m := map[string]any{"a": 1}
	for _, in := range []any{nil, nilUser, m, &m, []int{1}, 3} {
		if err := v.Validate(in); err != nil {
			t.Errorf("Validate(%T): %v", in, err)
		}
	}
}

func TestWrapAndEngine(t *testing.T) {
	eng := validator.New()
	eng.RegisterTagNameFunc(playground.TagName)
	if err := eng.RegisterValidation("even", func(fl validator.FieldLevel) bool {
		return fl.Field().Int()%2 == 0
	}); err != nil {
		t.Fatal(err)
	}
	v := playground.Wrap(eng)
	if v.Engine() != eng {
		t.Fatal("Engine did not return the wrapped validator")
	}
	type in struct {
		N int `json:"n" validate:"even"`
	}
	got := fields(t, v.Validate(in{N: 3}))
	if len(got) != 1 || got[0] != (chichi.FieldError{Field: "n", Reason: "even"}) {
		t.Fatalf("got %v", got)
	}
}

func TestBinderIntegration(t *testing.T) {
	b := chichi.New(chichi.WithValidator(playground.NewValidator()))
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"x","age":30,"address":{"city":"y"}}`))
	r.Header.Set("Content-Type", "application/json")
	var u createUser
	got := fields(t, b.BindJSON(r, &u))
	if len(got) != 1 || got[0].Field != "email" {
		t.Fatalf("got %v", got)
	}
}
