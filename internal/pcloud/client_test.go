package pcloud

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestFolderMetadataAndTextErrors(t *testing.T) {
	c, _ := New("eu", "token")
	c.http.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		return response(`{"result":0,"metadata":{"folderid":10,"isfolder":true,"ismine":true,"contents":[{"fileid":11,"name":"a.txt","ismine":true,"parentfolderid":10}]}}`), nil
	})
	folder, err := c.ListFolder(context.Background(), 10)
	if err != nil || folder.FolderID != 10 || len(folder.Contents) != 1 || folder.Contents[0].FileID != 11 {
		t.Fatal("folder decoding failed", err)
	}
	c.http.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		res := response("sensitive provider error")
		res.Header.Set("X-Error", "2000")
		return res, nil
	})
	if _, err := c.Text(context.Background(), 11); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatal("text API error exposed as contents")
	}
}
func TestMultipartSafety(t *testing.T) {
	c, _ := New("eu", "token")
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatal(err)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		fields := map[string]string{}
		fileSeen := false
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(part)
			if part.FileName() != "" {
				fileSeen = true
				if string(b) != "hello" || fields["renameifexists"] != "1" || fields["nopartial"] != "1" || fields["access_token"] != "token" {
					t.Fatal("unsafe multipart upload")
				}
			} else {
				if fileSeen {
					t.Fatal("parameters after file")
				}
				fields[part.FormName()] = string(b)
			}
		}
		if !fileSeen {
			t.Fatal("file missing")
		}
		return response(`{"result":0,"metadata":[{"fileid":12,"name":"a (2).txt","size":5,"ismine":true}]}`), nil
	})
	m, err := c.Upload(context.Background(), 10, "a.txt", []byte("hello"))
	if err != nil || m.Name != "a (2).txt" {
		t.Fatal("upload failed", err)
	}
}
func FuzzDecode(f *testing.F) {
	f.Add([]byte(`{"result":0,"userid":7}`))
	f.Add([]byte(`{"result":2000}`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			return
		}
		var out map[string]any
		_ = decode(data, &out)
	})
}
func TestTokenAndCopySafety(t *testing.T) {
	c, _ := New("eu", "sentinel-token")
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "eapi.pcloud.com" || r.URL.RawQuery != "" || r.Method != "POST" {
			t.Fatal("unsafe API request")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("access_token") != "sentinel-token" || r.Form.Get("noover") != "1" {
			t.Fatal("token or no-overwrite missing")
		}
		return response(`{"result":0,"metadata":{"fileid":3,"ismine":true}}`), nil
	})
	if _, err := c.Copy(context.Background(), 1, 2, "new.txt"); err != nil {
		t.Fatal(err)
	}
}
func TestErrorsAreRedactedAndMissingResultRejected(t *testing.T) {
	for _, body := range []string{`{"result":2000,"error":"sentinel-token"}`, `{"userid":7}`, `not json sentinel-token`} {
		c, _ := New("us", "sentinel-token")
		c.http.Transport = transportFunc(func(*http.Request) (*http.Response, error) { return response(body), nil })
		_, err := c.Whoami(context.Background())
		if err == nil || strings.Contains(err.Error(), "sentinel-token") {
			t.Fatalf("unsafe error %v", err)
		}
	}
}
func TestRegionAndRedirects(t *testing.T) {
	for _, region := range []string{"", "EU", "https://evil.invalid", "localhost"} {
		if _, err := New(region, "token"); err == nil {
			t.Fatal("invalid region accepted")
		}
	}
	c, _ := New("eu", "token")
	if c.http.CheckRedirect(&http.Request{}, nil) == nil {
		t.Fatal("redirect allowed")
	}
}
func TestResponseLimit(t *testing.T) {
	c, _ := New("eu", "token")
	c.http.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		return response(strings.Repeat(" ", MaxResponseBytes+1)), nil
	})
	if _, err := c.Whoami(context.Background()); err == nil {
		t.Fatal("oversize response accepted")
	}
}
