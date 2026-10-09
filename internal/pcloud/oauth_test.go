package pcloud

import (
	"net/url"
	"testing"
)

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
