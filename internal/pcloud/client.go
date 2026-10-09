// Package pcloud implements the fixed-host pCloud API transport. All parameters,
// including OAuth tokens, travel in POST bodies, never in URLs.
package pcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cyrano-janus/pcloud-mcp/internal/model"
	"github.com/cyrano-janus/pcloud-mcp/internal/netguard"
)

const MaxResponseBytes = 4 << 20
const MaxTextBytes = 128 << 10
const MaxUploadBytes = 4 << 20

var ErrTransport = errors.New("pCloud request failed; for writes the outcome may be unknown; do not retry automatically")
var ErrResponse = errors.New("pCloud response invalid or exceeds the size limit")

type APIError struct{ Code int }

func (e APIError) Error() string { return fmt.Sprintf("pCloud rejected request (code %d)", e.Code) }

type Client struct {
	base, token string
	http        *http.Client
}

func New(region, token string) (*Client, error) {
	hosts := map[string]string{"eu": "https://eapi.pcloud.com", "us": "https://api.pcloud.com"}
	base, ok := hosts[region]
	if !ok || token == "" || len(token) > 4096 || strings.ContainsAny(token, "\r\n") {
		return nil, errors.New("valid pCloud region and OAuth token required")
	}
	return &Client{base: base, token: token, http: netguard.Client(20 * time.Second)}, nil
}
func (c *Client) request(ctx context.Context, method, contentType string, body io.Reader, limit int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/"+method, body)
	if err != nil {
		return nil, ErrTransport
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "pcloud-mcp/0.3.0")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, ErrTransport
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, ErrTransport
	}
	if x := res.Header.Get("X-Error"); x != "" && x != "0" {
		code, err := strconv.Atoi(x)
		if err != nil {
			return nil, ErrResponse
		}
		return nil, APIError{code}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, int64(limit)+1))
	if err != nil || len(data) > limit {
		return nil, ErrResponse
	}
	return data, nil
}
func (c *Client) call(ctx context.Context, method string, params url.Values, out any) error {
	params.Set("access_token", c.token)
	data, err := c.request(ctx, method, "application/x-www-form-urlencoded", strings.NewReader(params.Encode()), MaxResponseBytes)
	if err != nil {
		return err
	}
	return decode(data, out)
}
func decode(data []byte, out any) error {
	var status struct {
		Result *int `json:"result"`
	}
	if json.Unmarshal(data, &status) != nil || status.Result == nil {
		return ErrResponse
	}
	if *status.Result != 0 {
		return APIError{*status.Result}
	}
	if json.Unmarshal(data, out) != nil {
		return ErrResponse
	}
	return nil
}
func (c *Client) Whoami(ctx context.Context) (model.Identity, error) {
	var out model.Identity
	err := c.call(ctx, "userinfo", url.Values{}, &out)
	if err == nil && out.UserID <= 0 {
		err = ErrResponse
	}
	return out, err
}
func (c *Client) meta(ctx context.Context, method string, params url.Values) (model.Metadata, error) {
	var out struct {
		Metadata model.Metadata `json:"metadata"`
	}
	err := c.call(ctx, method, params, &out)
	return out.Metadata, err
}
func id(v int64) string { return strconv.FormatInt(v, 10) }
func (c *Client) ListFolder(ctx context.Context, folderID int64) (model.Metadata, error) {
	return c.meta(ctx, "listfolder", url.Values{"folderid": {id(folderID)}, "recursive": {"0"}})
}
func (c *Client) Stat(ctx context.Context, fileID int64) (model.Metadata, error) {
	return c.meta(ctx, "stat", url.Values{"fileid": {id(fileID)}})
}
func (c *Client) CreateFolder(ctx context.Context, folderID int64, name string) (model.Metadata, error) {
	return c.meta(ctx, "createfolder", url.Values{"folderid": {id(folderID)}, "name": {name}})
}
func (c *Client) Copy(ctx context.Context, fileID, folderID int64, name string) (model.Metadata, error) {
	return c.meta(ctx, "copyfile", url.Values{"fileid": {id(fileID)}, "tofolderid": {id(folderID)}, "toname": {name}, "noover": {"1"}})
}
func (c *Client) Text(ctx context.Context, fileID int64) (string, error) {
	p := url.Values{"access_token": {c.token}, "fileid": {id(fileID)}, "toencoding": {"utf-8"}}
	data, err := c.request(ctx, "gettextfile", "application/x-www-form-urlencoded", strings.NewReader(p.Encode()), MaxTextBytes)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", errors.New("file is not supported UTF-8 text")
	}
	return string(data), nil
}
func (c *Client) Upload(ctx context.Context, folderID int64, name string, data []byte) (model.Metadata, error) {
	if len(data) > MaxUploadBytes {
		return model.Metadata{}, errors.New("upload exceeds limit")
	}
	// Stream the multipart envelope; the already size-checked client artifact is
	// bounded in memory. Never stage files in an arbitrary local filesystem path.
	reader, writer := io.Pipe()
	mw := multipart.NewWriter(writer)
	done := make(chan error, 1)
	go func() {
		err := func() error {
			for _, field := range [][2]string{{"access_token", c.token}, {"folderid", id(folderID)}, {"renameifexists", "1"}, {"nopartial", "1"}} {
				if err := mw.WriteField(field[0], field[1]); err != nil {
					return err
				}
			}
			part, err := mw.CreateFormFile("file", name)
			if err != nil {
				return err
			}
			if _, err = io.Copy(part, bytes.NewReader(data)); err != nil {
				return err
			}
			return mw.Close()
		}()
		_ = writer.CloseWithError(err)
		done <- err
	}()
	result, err := c.request(ctx, "uploadfile", mw.FormDataContentType(), reader, MaxResponseBytes)
	_ = reader.Close()
	writeErr := <-done
	if err != nil {
		return model.Metadata{}, err
	}
	if writeErr != nil {
		return model.Metadata{}, ErrTransport
	}
	var out struct {
		Metadata []model.Metadata `json:"metadata"`
	}
	if err := decode(result, &out); err != nil {
		return model.Metadata{}, err
	}
	if len(out.Metadata) != 1 {
		return model.Metadata{}, ErrResponse
	}
	return out.Metadata[0], nil
}
