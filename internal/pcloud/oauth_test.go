package pcloud

import (
	"encoding/json"
	"net/url"
	"testing"
)

func TestOAuthCredentialIdentityFormats(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		valid        bool
	}{
		{"documented uid", `"uid":7`, true},
		{"SDK userid", `"userid":7`, true},
		{"matching aliases", `"uid":7,"userid":7`, true},
		{"conflicting aliases", `"uid":7,"userid":8`, false},
		{"missing", `"other":7`, false},
		{"zero", `"userid":0`, false},
		{"negative", `"userid":-1`, false},
		{"string", `"userid":"7"`, false},
		{"null alias", `"uid":null,"userid":7`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c OAuthCredential
			err := json.Unmarshal([]byte(`{"access_token":"test-token","token_type":"bearer",`+tc.fields+`}`), &c)
			if (err == nil) != tc.valid || (tc.valid && (c.UserID != 7 || c.Token != "test-token" || c.Type != "bearer")) {
				t.Fatal("incorrect token identity parsing", err)
			}
		})
	}
}

func TestCallbackBinding(t *testing.T) {
	good := url.Values{"state": {"expected"}, "code": {"code"}, "hostname": {"eapi.pcloud.com"}, "locationid": {"2"}}
	region, code, err := ValidateCallback(good, "expected")
	if err != nil || region != "eu" || code != "code" {
		t.Fatal("valid callback failed", err)
	}
	for _, key := range []string{"state", "hostname", "locationid"} {
		v := url.Values{}
		for k, values := range good {
			v[k] = append([]string{}, values...)
		}
		v.Set(key, "wrong")
		if _, _, err := ValidateCallback(v, "expected"); err == nil {
			t.Fatalf("invalid %s accepted", key)
		}
	}
	good.Add("state", "expected")
	if _, _, err := ValidateCallback(good, "expected"); err == nil {
		t.Fatal("duplicate state accepted")
	}
}

func FuzzOAuthCallback(f *testing.F) {
	f.Add("expected", "code", "eapi.pcloud.com", "2")
	f.Add("wrong", "code", "api.pcloud.com", "1")
	f.Fuzz(func(t *testing.T, state, code, host, location string) {
		q := url.Values{"state": {state}, "code": {code}, "hostname": {host}, "locationid": {location}}
		region, _, err := ValidateCallback(q, "expected")
		if err == nil && (state != "expected" || code == "" || len(code) > 4096 || !((region == "eu" && host == "eapi.pcloud.com" && location == "2") || (region == "us" && host == "api.pcloud.com" && location == "1"))) {
			t.Fatal("invalid callback accepted")
		}
	})
}
