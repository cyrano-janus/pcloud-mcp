package pcloud

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/url"
	"strings"
)

type OAuthCredential struct {
	Token  string `json:"access_token"`
	Type   string `json:"token_type"`
	UserID int64  `json:"uid"`
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
