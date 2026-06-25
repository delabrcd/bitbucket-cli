package api

import (
	"encoding/json"
	"testing"
)

func TestTypedFieldValue(t *testing.T) {
	cases := []struct {
		in   string
		want interface{}
	}{
		{"true", true},
		{"false", false},
		{"null", nil},
		{"42", int64(42)},
		{"3.14", 3.14},
		{"hello", "hello"},
		{"v2.0.0", "v2.0.0"},
	}
	for _, c := range cases {
		got, err := typedFieldValue(c.in)
		if err != nil {
			t.Fatalf("typedFieldValue(%q) returned error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("typedFieldValue(%q) = %#v (%T), want %#v (%T)", c.in, got, got, c.want, c.want)
		}
	}
}

func TestBuildBodyFieldsAreJSON(t *testing.T) {
	apiOptions.RawFields = []string{"title=hello"}
	apiOptions.Fields = []string{"count=3", "enabled=true", "draft=null"}
	defer func() { apiOptions.RawFields = nil; apiOptions.Fields = nil }()

	body, payloadType, err := buildBody()
	if err != nil {
		t.Fatalf("buildBody returned error: %v", err)
	}
	if payloadType != "application/json" {
		t.Errorf("payloadType = %q, want application/json", payloadType)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got["title"] != "hello" {
		t.Errorf("title = %#v, want string hello", got["title"])
	}
	if got["count"] != float64(3) { // JSON numbers decode to float64
		t.Errorf("count = %#v, want number 3", got["count"])
	}
	if got["enabled"] != true {
		t.Errorf("enabled = %#v, want bool true", got["enabled"])
	}
	if v, ok := got["draft"]; !ok || v != nil {
		t.Errorf("draft = %#v (present=%v), want null", v, ok)
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	cases := map[string]string{
		"user":                               "/user",
		"/user":                              "/user",
		"https://api.bitbucket.org/2.0/user": "https://api.bitbucket.org/2.0/user",
	}
	for in, want := range cases {
		if got := normalizeEndpoint(in); got != want {
			t.Errorf("normalizeEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
}
