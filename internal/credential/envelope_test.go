package credential

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEnvelopeIntegrityAndBinding(t *testing.T) {
	key := strings.Repeat("ab", 32)
	data := Data{Token: "secret-pcloud-token", Region: "eu", UserID: 7}
	encrypted, err := Seal(key, data)
	if err != nil || strings.Contains(encrypted, data.Token) {
		t.Fatal("token not protected", err)
	}
	got, err := Open(key, encrypted)
	if err != nil || got != data {
		t.Fatal("round trip failed", err)
	}
	second, _ := Seal(key, data)
	if second == encrypted {
		t.Fatal("nonce reused")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(encrypted[3:])
	raw[len(raw)-1] ^= 1
	tampered := "v1." + base64.RawURLEncoding.EncodeToString(raw)
	for _, value := range []string{tampered, "v2." + encrypted[3:], "invalid"} {
		if _, err := Open(key, value); err == nil {
			t.Fatal("modified envelope accepted")
		}
	}
	if _, err := Open(strings.Repeat("cd", 32), encrypted); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := Seal("weak", data); err == nil {
		t.Fatal("weak key accepted")
	}
	for _, d := range []Data{{Token: "x", Region: "bad", UserID: 7}, {Token: "x", Region: "eu", UserID: 0}, {Token: "x\n", Region: "eu", UserID: 7}} {
		if _, err := Seal(key, d); err == nil {
			t.Fatal("invalid credential accepted")
		}
	}
}

func FuzzEnvelope(f *testing.F) {
	f.Add("invalid")
	f.Add("v1.AAAAAAAA")
	key := strings.Repeat("ab", 32)
	valid, _ := Seal(key, Data{Token: "fixture", Region: "eu", UserID: 7})
	f.Add(valid)
	f.Fuzz(func(t *testing.T, value string) {
		data, err := Open(key, value)
		if err == nil && !validDataForFuzz(data) {
			t.Fatal("invalid decrypted data")
		}
	})
}
func validDataForFuzz(data Data) bool {
	return data.Token != "" && data.UserID > 0 && (data.Region == "eu" || data.Region == "us")
}
