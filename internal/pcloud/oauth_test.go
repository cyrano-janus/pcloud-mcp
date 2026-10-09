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
