package pcloud

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Video consumes the provider's temporary capability internally. It is never
// returned to the MCP client, logged, or used with the account's Authorization.
func (c *Client) Video(ctx context.Context, fileID int64, dst io.Writer, size int64) error {
	if size <= 0 || size > 128<<20 {
		return ErrResponse
	}
	var link struct {
		Hosts   []string `json:"hosts"`
		Path    string   `json:"path"`
		Expires string   `json:"expires"`
	}
	if err := c.call(ctx, "getfilelink", url.Values{"fileid": {id(fileID)}, "skipfilename": {"1"}}, &link); err != nil {
		return err
	}
	if len(link.Hosts) == 0 || len(link.Hosts) > 16 {
		return ErrResponse
	}
	host := link.Hosts[0]
	// Only provider-owned DNS names, no userinfo, ports, IPs or URL syntax.
	if !strings.HasSuffix(host, ".pcloud.com") || len(host) > 253 {
		return ErrResponse
	}
	for _, r := range host {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-') {
			return ErrResponse
		}
	}
	expiry, err := time.Parse(time.RFC1123Z, link.Expires)
	if err != nil || !expiry.After(time.Now()) {
		return ErrResponse
	}
	u, err := url.Parse("https://" + host + link.Path)
	if err != nil || !strings.HasPrefix(link.Path, "/") || strings.HasPrefix(link.Path, "//") || strings.ContainsAny(link.Path, "\\\r\n") || u.Host != host || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return ErrResponse
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ErrTransport
	}
	res, err := c.http.Do(req)
	if err != nil {
		return ErrTransport
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return &HTTPError{StatusCode: res.StatusCode}
	}
	if res.ContentLength > size {
		return ErrResponse
	}
	n, err := io.Copy(dst, io.LimitReader(res.Body, size+1))
	if err != nil || n != size {
		return ErrResponse
	}
	return nil
}
