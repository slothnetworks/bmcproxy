package amimegarac

import (
	"encoding/json"
	"testing"
)

// sensor mirrors the parts of /api/sensors that drivers care about.
type sensor struct {
	ID       NullableInt   `json:"id"`
	Name     FlexString    `json:"name"`
	Reading  NullableFloat `json:"reading"`
	LNC      NullableFloat `json:"lower_non_critical_threshold"`
	HNC      NullableFloat `json:"higher_non_critical_threshold"`
	HC       NullableFloat `json:"higher_critical_threshold"`
	Unit     FlexString    `json:"unit"`
	Missing  NullableFloat `json:"not_there"`
	Settable NullableInt   `json:"settable_flag"`
}

func TestNADecodingFromHARSensors(t *testing.T) {
	var ss []sensor
	if err := Decode([]byte(harSensorsB650), &ss); err != nil {
		t.Fatal(err)
	}
	if len(ss) != 2 {
		t.Fatalf("%d sensors", len(ss))
	}
	s := ss[0]
	if s.Name != "VOLT_3VSB" || !s.Reading.Valid || s.Reading.V != 3.34 {
		t.Errorf("reading %+v", s)
	}
	if s.LNC.Valid || s.LNC.Ptr() != nil || s.HNC.Valid {
		t.Errorf(`"NA" thresholds decoded as valid: %+v %+v`, s.LNC, s.HNC)
	}
	if !s.HC.Valid || *s.HC.Ptr() != 3.72 {
		t.Errorf("HC %+v", s.HC)
	}
	if s.Missing.Valid {
		t.Error("missing field valid")
	}
	if s.Settable.V != 13878 {
		t.Errorf("settable %+v", s.Settable)
	}
}

func TestNullableFloatForms(t *testing.T) {
	cases := []struct {
		in    string
		v     float64
		valid bool
	}{
		{`1.5`, 1.5, true},
		{`"60"`, 60, true},
		{`" 2.5 "`, 2.5, true},
		{`"NA"`, 0, false},
		{`"N\/A"`, 0, false},
		{`"n/a"`, 0, false},
		{`""`, 0, false},
		{`null`, 0, false},
		{`"garbage"`, 0, false},
		{`0`, 0, true},
		{`-2147476070`, -2147476070, true},
	}
	for _, c := range cases {
		var n NullableFloat
		if err := json.Unmarshal([]byte(c.in), &n); err != nil {
			t.Errorf("%s: %v", c.in, err)
			continue
		}
		if n.Valid != c.valid || n.V != c.v {
			t.Errorf("%s -> %+v, want %v/%v", c.in, n, c.v, c.valid)
		}
	}
	b, _ := json.Marshal(struct{ A, B NullableFloat }{NullableFloat{1, true}, NullableFloat{}})
	if string(b) != `{"A":1,"B":null}` {
		t.Errorf("marshal %s", b)
	}
}

func TestNullableIntAndFlex(t *testing.T) {
	var v struct {
		A NullableInt `json:"a"`
		B NullableInt `json:"b"`
		C NullableInt `json:"c"`
		D NullableInt `json:"d"`
		E FlexString  `json:"e"`
		F FlexString  `json:"f"`
		G FlexBool    `json:"g"`
		H FlexBool    `json:"h"`
		I FlexBool    `json:"i"`
		J FlexString  `json:"j"`
	}
	in := `{"a":"60","b":"NA","c":3.0,"d":"0x1F","e":42,"f":null,"g":1,"h":"0","i":"enabled","j":"Thu Jan 13 06:21:00 2022\\n"}`
	if err := json.Unmarshal([]byte(in), &v); err != nil {
		t.Fatal(err)
	}
	if v.A.V != 60 || !v.A.Valid || v.B.Valid || v.C.V != 3 || v.D.V != 31 {
		t.Errorf("ints %+v %+v %+v %+v", v.A, v.B, v.C, v.D)
	}
	if v.E != "42" || v.F != "" {
		t.Errorf("flexstring %q %q", v.E, v.F)
	}
	if !v.G || v.H || !v.I {
		t.Errorf("flexbool %v %v %v", v.G, v.H, v.I)
	}
	if v.J.Clean() != "Thu Jan 13 06:21:00 2022" {
		t.Errorf("clean %q", v.J.Clean())
	}
}

func TestDecodePort(t *testing.T) {
	cases := []struct {
		in       int64
		port     int
		disabled bool
	}{
		{443, 443, false},
		{-1, 0, true},
		{-2147476070, 7578, true},
		{-2147478528, 5120, true},
	}
	for _, c := range cases {
		p, d := DecodePort(c.in)
		if p != c.port || d != c.disabled {
			t.Errorf("DecodePort(%d) = %d,%v", c.in, p, d)
		}
	}
}

func TestParseAPIError(t *testing.T) {
	e := ParseAPIError(401, nil, []byte(ccInvalidAuth))
	if !e.HasCC || e.CC != 7 || e.Message != "Invalid Authentication" {
		t.Errorf("%+v", e)
	}
	if !isAuthGateReject([]byte(ccInvalidAuth)) || isAuthGateReject([]byte("<html>")) || isAuthGateReject(nil) {
		t.Error("isAuthGateReject")
	}
	e = ParseAPIError(404, nil, []byte(`{ "error": "Invalid API Call", "code": 1010 }`))
	if e.Code != 1010 || e.HasCC {
		t.Errorf("%+v", e)
	}
}
