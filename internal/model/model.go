// Package model defines provider data independently of MCP and HTTP transports.
package model

import "encoding/json"

type Identity struct {
	UserID    int64  `json:"userid"`
	Email     string `json:"email"`
	Quota     int64  `json:"quota"`
	UsedQuota int64  `json:"usedquota"`
}
type Metadata struct {
	FileID         int64  `json:"fileid,omitempty"`
	FolderID       int64  `json:"folderid"`
	ParentFolderID int64  `json:"parentfolderid"`
	Name           string `json:"name"`
	IsFolder       bool   `json:"isfolder"`
	IsMine         bool   `json:"ismine"`
	Size           int64  `json:"size"`
	ContentType    string `json:"contenttype,omitempty"`
	Created        string `json:"created,omitempty"`
	Modified       string `json:"modified,omitempty"`
	Hash           uint64 `json:"hash,omitempty"`
	// Provider folder contents are internal only. MCP returns an explicitly
	// filtered and bounded entries list instead of recursively serializing them.
	Contents []Metadata `json:"-"`
}

func (m *Metadata) UnmarshalJSON(data []byte) error {
	type fields Metadata
	var wire struct {
		*fields
		Contents []Metadata `json:"contents"`
	}
	wire.fields = (*fields)(m)
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	m.Contents = wire.Contents
	return nil
}
