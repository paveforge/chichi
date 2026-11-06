package structmap

import (
	"errors"
	"net"
	"reflect"
	"testing"
	"time"
)

type Level string

type Embedded struct {
	Page int `q:"page"`
}

type embeddedLower struct {
	Size int `q:"size"`
}

type all struct {
	Embedded
	embeddedLower
	Level // embedded non-struct: ignored

	S    string     `q:"s"`
	B    bool       `q:"b"`
	I    int        `q:"i"`
	I8   int8       `q:"i8"`
	I16  int16      `q:"i16"`
	I32  int32      `q:"i32"`
	I64  int64      `q:"i64"`
	U    uint       `q:"u"`
	U8   uint8      `q:"u8"`
	U16  uint16     `q:"u16"`
	U32  uint32     `q:"u32"`
	U64  uint64     `q:"u64"`
	F32  float32    `q:"f32"`
	F64  float64    `q:"f64"`
	T    time.Time  `q:"t"`
	IP   net.IP     `q:"ip"`
	P    *int       `q:"p"`
	PT   *time.Time `q:"pt"`
	Tags []string   `q:"tags"`
	Nums []int      `q:"nums"`
	PNum []*int     `q:"pnum"`

	Named     string `q:",omitempty"`
	Skipped   string `q:"-"`
	unexp     string `q:"unexp"` //nolint:unused // proves unexported fields are skipped
	NoTag     string
	Missing   int `q:"missing"`
	Untouched int `q:"untouched"`
}

func TestDecodeAllTypes(t *testing.T) {
	values := map[string][]string{
		"page": {"2"}, "size": {"50"},
		"s": {"hi"}, "b": {"true"},
		"i": {"-1"}, "i8": {"-8"}, "i16": {"-16"}, "i32": {"-32"}, "i64": {"-64"},
		"u": {"1"}, "u8": {"8"}, "u16": {"16"}, "u32": {"32"}, "u64": {"64"},
		"f32": {"1.5"}, "f64": {"2.5"},
		"t": {"2026-01-02T03:04:05Z"}, "ip": {"10.0.0.1"},
		"p": {"7"}, "pt": {"2026-01-02T03:04:05Z"},
		"tags": {"a", "b"}, "nums": {"1", "2"}, "pnum": {"3"},
		"Named": {"n"}, "-": {"x"}, "Skipped": {"x"}, "unexp": {"x"}, "NoTag": {"x"},
		"untouched": {},
	}
	var got all
	got.Untouched = 9
	if err := Decode(values, &got, "q"); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	seven, three := 7, 3
	want := all{
		Embedded: Embedded{Page: 2}, embeddedLower: embeddedLower{Size: 50},
		S: "hi", B: true,
		I: -1, I8: -8, I16: -16, I32: -32, I64: -64,
		U: 1, U8: 8, U16: 16, U32: 32, U64: 64,
		F32: 1.5, F64: 2.5,
		T: ts, IP: net.ParseIP("10.0.0.1"), P: &seven, PT: &ts,
		Tags: []string{"a", "b"}, Nums: []int{1, 2}, PNum: []*int{&three},
		Named: "n", Untouched: 9,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestDecodeFieldErrors(t *testing.T) {
	tests := []struct {
		key, value, reason string
	}{
		{"b", "maybe", "invalid boolean"},
		{"i", "x", "invalid integer"},
		{"i8", "300", "invalid integer"},
		{"u", "-1", "invalid unsigned integer"},
		{"f64", "x", "invalid number"},
		{"t", "yesterday", "invalid value"},
		{"p", "x", "invalid integer"},
		{"nums", "x", "invalid integer"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			var dst all
			err := Decode(map[string][]string{tt.key: {tt.value}}, &dst, "q")
			var errs Errors
			if !errors.As(err, &errs) {
				t.Fatalf("want Errors, got %v", err)
			}
			if want := (Errors{{Field: tt.key, Reason: tt.reason}}); !reflect.DeepEqual(errs, want) {
				t.Fatalf("got %v, want %v", errs, want)
			}
		})
	}
}

func TestDecodeCollectsAllErrors(t *testing.T) {
	var dst all
	err := Decode(map[string][]string{"i": {"x"}, "b": {"x"}}, &dst, "q")
	want := `field "b": invalid boolean; field "i": invalid integer`
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %s", err, want)
	}
}

func TestDecodeInvalidTarget(t *testing.T) {
	var nilPtr *all
	n := 1
	for name, dst := range map[string]any{"non-pointer": all{}, "nil": nilPtr, "non-struct": &n} {
		t.Run(name, func(t *testing.T) {
			if err := Decode(nil, dst, "q"); !errors.Is(err, ErrNotStruct) {
				t.Fatalf("got %v, want ErrNotStruct", err)
			}
		})
	}
}

type unsupported struct {
	M map[string]string `q:"m"`
}

type unsupportedSlice struct {
	S [][]int `q:"s"`
}

type unsupportedEmbedded struct {
	unsupported
}

func TestDecodeUnsupportedType(t *testing.T) {
	for name, dst := range map[string]any{
		"map": &unsupported{}, "nested slice": &unsupportedSlice{}, "embedded": &unsupportedEmbedded{},
	} {
		t.Run(name, func(t *testing.T) {
			err := Decode(nil, dst, "q")
			var ute *UnsupportedTypeError
			if !errors.As(err, &ute) {
				t.Fatalf("got %v, want UnsupportedTypeError", err)
			}
			if ute.Error() == "" {
				t.Fatal("empty message")
			}
		})
	}
}
