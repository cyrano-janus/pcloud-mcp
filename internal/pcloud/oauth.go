package pcloud

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

type OAuthCredential struct {
	Token  string `json:"access_token"`
	Type   string `json:"token_type"`
	UserID int64  `json:"uid"`
}

// pCloud's token-method documentation uses uid, while its official JavaScript
// SDK consumes userid. Accept either integer spelling, but reject ambiguity.
func (c *OAuthCredential) UnmarshalJSON(data []byte) error {
	var wire struct {
		Token  string          `json:"access_token"`
		Type   string          `json:"token_type"`
		UID    json.RawMessage `json:"uid"`
		UserID json.RawMessage `json:"userid"`
	}
	if json.Unmarshal(data, &wire) != nil {
		return ErrResponse
	}
	var owner int64
	for _, raw := range []json.RawMessage{wire.UID, wire.UserID} {
		if len(raw) == 0 {
			continue
		}
		var id int64
		if json.Unmarshal(raw, &id) != nil || id <= 0 || (owner > 0 && owner != id) {
			return ErrResponse
		}
		owner = id
	}
	if owner <= 0 {
		return ErrResponse
	}
	*c = OAuthCredential{Token: wire.Token, Type: wire.Type, UserID: owner}
	return nil
}

func AuthorizationURL(clientID, redirect, state string) string {
	return "https://my.pcloud.com/oauth2/authorize?" + url.Values{"client_id": {clientID}, "response_type": {"code"}, "redirect_uri": {redirect}, "state": {state}}.Encode()
}
func ValidateCallback(q url.Values, state string) (region, code string, err error) {
	invalid := errors.New("OAuth callback rejected")
	for _, key := range []string{"state", "code", "hostname", "locationid"} {
		if len(q[key]) != 1 {
			return "", "", invalid
		}
	}
	if len(q.Get("state")) != len(state) || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 || q.Get("code") == "" || len(q.Get("code")) > 4096 || q.Get("error") != "" {
		return "", "", invalid
	}
	switch {
	case q.Get("locationid") == "1" && q.Get("hostname") == "api.pcloud.com":
		region = "us"
	case q.Get("locationid") == "2" && q.Get("hostname") == "eapi.pcloud.com":
		region = "eu"
	default:
		return "", "", invalid
	}
	return region, q.Get("code"), nil
}
func ExchangeCode(ctx context.Context, region, clientID, clientSecret, code string) (OAuthCredential, error) {
	var out OAuthCredential
	if clientID == "" || clientSecret == "" || code == "" {
		return out, errors.New("pCloud app credentials and authorization code required")
	}
	c, err := New(region, "oauth-exchange")
	if err != nil {
		return out, err
	}
	body := url.Values{"client_id": {clientID}, "client_secret": {clientSecret}, "code": {code}}
	data, err := c.request(ctx, "oauth2_token", "application/x-www-form-urlencoded", strings.NewReader(body.Encode()), 65536)
	if err != nil {
		return out, err
	}
	if err := decode(data, &out); err != nil {
		return out, err
	}
	if out.Token == "" || len(out.Token) > 4096 || strings.ContainsAny(out.Token, "\r\n") || !strings.EqualFold(out.Type, "bearer") || out.UserID <= 0 {
		return OAuthCredential{}, ErrResponse
	}
	return out, nil
}
