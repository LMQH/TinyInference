package admin

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeModelName(t *testing.T) {
	cases := []struct {
		body  string
		valid bool
	}{
		{`{"public_model_id":"team/model-v2","expected_public_model_id":"openbmb/MiniCPM5-2B-Q4_K_M"}`, true},
		{`{"public_model_id":"","expected_public_model_id":"old"}`, false},
		{`{"public_model_id":"模型","expected_public_model_id":"old"}`, false},
		{`{"public_model_id":"new","public_model_id":"other","expected_public_model_id":"old"}`, false},
		{`{"public_model_id":"new","expected_public_model_id":"old","extra":1}`, false},
		{`{"public_model_id":"new","expected_public_model_id":"old"} {}`, false},
	}
	for _, tc := range cases {
		r := httptest.NewRequest("POST", "/admin/v1/model/name", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		_, _, ok := decodeModelName(r)
		if ok != tc.valid {
			t.Fatalf("body %q: got %v, want %v", tc.body, ok, tc.valid)
		}
	}
}
