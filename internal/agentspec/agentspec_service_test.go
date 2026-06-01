package agentspec

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nacos-group/nacos-cli/internal/client"
)

func TestBuildResourceRelativePath(t *testing.T) {
	cases := []struct {
		name string
		res  *AgentSpecResource
		want string
	}{
		{"nil", nil, ""},
		{"type and basename", &AgentSpecResource{Type: "config", Name: "app.yaml"}, "config/app.yaml"},
		{"nested type", &AgentSpecResource{Type: "skills/my-skill", Name: "SKILL.md"}, "skills/my-skill/SKILL.md"},
		{"name already prefixed", &AgentSpecResource{Type: "config", Name: "config/app.yaml"}, "config/app.yaml"},
		{"no type", &AgentSpecResource{Type: "", Name: "Dockerfile"}, "Dockerfile"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildResourceRelativePath(tc.res)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAgentSpecListItem_UnmarshalJSON(t *testing.T) {
	// Test that nullable fields are handled correctly
	jsonData := `{
		"name": "test-worker",
		"description": null,
		"enable": true,
		"labels": {},
		"editingVersion": "v1",
		"reviewingVersion": null,
		"onlineCnt": 0,
		"updateTime": 1773990903499
	}`

	var item AgentSpecListItem
	if err := json.Unmarshal([]byte(jsonData), &item); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if item.Name != "test-worker" {
		t.Errorf("Expected name 'test-worker', got %q", item.Name)
	}
	if item.Description != nil {
		t.Errorf("Expected Description to be nil, got %v", *item.Description)
	}
	if !item.Enable {
		t.Error("Expected Enable to be true")
	}
	if item.EditingVersion == nil || *item.EditingVersion != "v1" {
		t.Errorf("Expected EditingVersion 'v1', got %v", item.EditingVersion)
	}
	if item.ReviewingVersion != nil {
		t.Errorf("Expected ReviewingVersion to be nil, got %v", item.ReviewingVersion)
	}
	if item.OnlineCnt != 0 {
		t.Errorf("Expected OnlineCnt 0, got %d", item.OnlineCnt)
	}
	if item.UpdateTime != 1773990903499 {
		t.Errorf("Expected UpdateTime 1773990903499, got %d", item.UpdateTime)
	}
}

func TestAgentSpecResource_Metadata(t *testing.T) {
	// Test that metadata can contain mixed types
	jsonData := `{
		"name": "app.yaml",
		"type": "config",
		"content": "key: value",
		"metadata": {
			"encoding": "base64",
			"uniformId": 1773990990177,
			"size": "1024"
		}
	}`

	var res AgentSpecResource
	if err := json.Unmarshal([]byte(jsonData), &res); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if res.Name != "app.yaml" {
		t.Errorf("Expected name 'app.yaml', got %q", res.Name)
	}
	if res.Type != "config" {
		t.Errorf("Expected type 'config', got %q", res.Type)
	}
	if res.Content != "key: value" {
		t.Errorf("Expected content 'key: value', got %q", res.Content)
	}

	if res.Metadata == nil {
		t.Fatal("Expected Metadata to be non-nil")
	}

	if enc, ok := res.Metadata["encoding"]; !ok || enc != "base64" {
		t.Errorf("Expected encoding 'base64', got %v", enc)
	}

	if uid, ok := res.Metadata["uniformId"]; !ok || uid != float64(1773990990177) {
		t.Errorf("Expected uniformId 1773990990177, got %v", uid)
	}

	if size, ok := res.Metadata["size"]; !ok || size != "1024" {
		t.Errorf("Expected size '1024', got %v", size)
	}
}

// TestUploadAgentSpecZipPaths verifies that:
// 1. ZIP entries have NO extra specName/ prefix (#46)
// 2. ZIP entries use forward slashes even on Windows (#26)
func TestUploadAgentSpecZipPaths(t *testing.T) {
	var zipData []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v3/admin/ai/agentspecs/upload" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("read uploaded file: %v", err)
		}
		defer file.Close()
		zipData, err = io.ReadAll(file)
		if err != nil {
			t.Fatalf("read upload body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create an agentspec directory with nested subdirectory
	specDir := filepath.Join(t.TempDir(), "my-agent")
	subDir := filepath.Join(specDir, "resources", "config")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "AGENTSPEC.md"), []byte("# My Agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "app.yaml"), []byte("key: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	nacosClient, err := client.NewNacosClient(
		strings.TrimPrefix(server.URL, "http://"),
		"test-ns",
		client.AuthTypeNone,
		"", "", "", "", "", "", "",
		"http",
	)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewAgentSpecService(nacosClient)
	if err := svc.UploadAgentSpec(specDir); err != nil {
		t.Fatal(err)
	}

	// Parse the ZIP and verify paths
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	expectedPaths := map[string]bool{
		"AGENTSPEC.md":              false,
		"resources/config/app.yaml": false,
	}

	for _, f := range reader.File {
		// Verify no backslashes (Windows path separator)
		if strings.Contains(f.Name, "\\") {
			t.Errorf("ZIP entry contains backslash: %q", f.Name)
		}
		// Verify no specName prefix
		if strings.HasPrefix(f.Name, "my-agent/") {
			t.Errorf("ZIP entry has unexpected specName prefix: %q", f.Name)
		}
		if _, ok := expectedPaths[f.Name]; ok {
			expectedPaths[f.Name] = true
		} else {
			t.Errorf("unexpected ZIP entry: %q", f.Name)
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("expected ZIP entry not found: %q", path)
		}
	}
}
