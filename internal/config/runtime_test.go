package config

import "testing"

func TestRuntimeConfig(t *testing.T) {
	base := map[string]string{"PCLOUD_ACCESS_TOKEN": "test-value", "PCLOUD_REGION": "eu", "PCLOUD_USER_ID": "7", "PCLOUD_ROOT_FOLDER_ID": "10"}
	for _, key := range []string{"PCLOUD_REGION", "PCLOUD_USER_ID", "PCLOUD_ROOT_FOLDER_ID"} {
		_, err := Load(func(k string) (string, bool) {
			if k == key {
				return "", false
			}
			v, ok := base[k]
			return v, ok
		})
		if err == nil {
			t.Fatalf("missing %s accepted", key)
		}
	}
	cfg, err := Load(func(k string) (string, bool) { v, ok := base[k]; return v, ok })
	if err != nil || cfg.EnableWrites || cfg.RootFolderID != 10 {
		t.Fatal("defaults invalid", err)
	}
	base["PCLOUD_MCP_TRANSPORT"] = "http"
	if _, err := Load(func(k string) (string, bool) { v, ok := base[k]; return v, ok }); err == nil {
		t.Fatal("unprotected HTTP accepted")
	}
}

func TestRenderProxyRequiresExplicitRuntime(t *testing.T) {
	values := map[string]string{"PCLOUD_MCP_TLS_PROXY": "render", "RENDER": "true", "PORT": "10000", "RENDER_EXTERNAL_URL": "https://pcloud-mcp.onrender.com"}
	load := func() (Config, error) {
		return Load(func(k string) (string, bool) { v, ok := values[k]; return v, ok })
	}
	cfg, err := load()
	if err != nil || !cfg.RenderProxy || cfg.Listen != "0.0.0.0:10000" || cfg.PublicURL != "https://pcloud-mcp.onrender.com/mcp" {
		t.Fatal("invalid Render config", err)
	}
	for _, key := range []string{"RENDER", "PORT"} {
		saved := values[key]
		delete(values, key)
		if _, err := load(); err == nil {
			t.Fatalf("missing %s accepted", key)
		}
		values[key] = saved
	}
	values["PCLOUD_MCP_TLS_PROXY"] = "arbitrary"
	if _, err := load(); err == nil {
		t.Fatal("unknown proxy accepted")
	}
}
