package pcloud

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestVideoDownloadBoundary(t *testing.T) {
	for _, host := range []string{"c1.pcloud.com", "pcloud.com.evil.test", "127.0.0.1", "c1.pcloud.com:443", "user@c1.pcloud.com", "c1.pcloud.com/evil"} {
		t.Run(host, func(t *testing.T) {
			c, _ := New("eu", "private-token")
			downloads := 0
			c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPost {
					if r.URL.RawQuery != "" {
						t.Fatal("token URL")
					}
					_ = r.ParseForm()
					if r.Form.Get("access_token") != "private-token" || r.Form.Get("fileid") != "11" {
						t.Fatal("wrong request")
					}
					return response(fmt.Sprintf(`{"result":0,"hosts":[%q],"path":"/opaque/clip.mp4","expires":%q}`, host, time.Now().Add(time.Hour).Format(time.RFC1123Z))), nil
				}
				downloads++
				if r.Header.Get("Authorization") != "" || strings.Contains(r.URL.String(), "private-token") {
					t.Fatal("credential forwarded")
				}
				return response("video"), nil
			})
			var out bytes.Buffer
			err := c.Video(context.Background(), 11, &out, 5)
			if host == "c1.pcloud.com" {
				if err != nil || out.String() != "video" {
					t.Fatal(err)
				}
			} else if err == nil || downloads != 0 {
				t.Fatal("unsafe host accepted")
			}
		})
	}
	c, _ := New("eu", "token")
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPost {
			return response(fmt.Sprintf(`{"result":0,"hosts":["c1.pcloud.com"],"path":"/opaque","expires":%q}`, time.Now().Add(time.Hour).Format(time.RFC1123Z))), nil
		}
		return response("oversized"), nil
	})
	if c.Video(context.Background(), 11, &bytes.Buffer{}, 5) == nil {
		t.Fatal("oversized video accepted")
	}
}
