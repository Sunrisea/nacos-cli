package prompt

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nacos-group/nacos-cli/internal/client"
)

func newTestNacosClient(serverURL string) (*client.NacosClient, error) {
	return client.NewNacosClient(
		strings.TrimPrefix(serverURL, "http://"),
		"test-ns",
		client.AuthTypeNone,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"http",
	)
}

func TestListPrompts(t *testing.T) {
	var listCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v3/admin/ai/prompt/list" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		listCalled = true
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if got := r.URL.Query().Get("namespaceId"); got != "test-ns" {
			t.Fatalf("namespaceId = %s, want test-ns", got)
		}
		if got := r.URL.Query().Get("pageNo"); got != "1" {
			t.Fatalf("pageNo = %s, want 1", got)
		}
		if got := r.URL.Query().Get("pageSize"); got != "20" {
			t.Fatalf("pageSize = %s, want 20", got)
		}

		resp := V3Response{Code: 0, Message: "success"}
		listData := PromptListResponse{
			TotalCount:     1,
			PageNumber:     1,
			PagesAvailable: 1,
			PageItems: []PromptListItem{
				{PromptKey: "test-prompt", Description: "A test prompt"},
			},
		}
		data, _ := json.Marshal(listData)
		resp.Data = data
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	nacosClient, err := newTestNacosClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	items, total, err := NewPromptService(nacosClient).ListPrompts("", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !listCalled {
		t.Fatal("list was not called")
	}
	if total != 1 {
		t.Fatalf("totalCount = %d, want 1", total)
	}
	if len(items) != 1 || items[0].PromptKey != "test-prompt" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListPromptsWithFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("promptKey"); got != "my-prompt" {
			t.Fatalf("promptKey = %s, want my-prompt", got)
		}
		if got := r.URL.Query().Get("search"); got != "accurate" {
			t.Fatalf("search = %s, want accurate", got)
		}
		resp := V3Response{Code: 0, Message: "success"}
		listData := PromptListResponse{TotalCount: 0, PageItems: []PromptListItem{}}
		data, _ := json.Marshal(listData)
		resp.Data = data
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	nacosClient, err := newTestNacosClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = NewPromptService(nacosClient).ListPrompts("my-prompt", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDescribePrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v3/admin/ai/prompt/governance" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if got := r.URL.Query().Get("promptKey"); got != "test-prompt" {
			t.Fatalf("promptKey = %s, want test-prompt", got)
		}

		detail := PromptDetail{
			PromptListItem: PromptListItem{
				PromptKey:      "test-prompt",
				Description:    "desc",
				EditingVersion: "0.0.1",
			},
			Versions: []PromptVersionSummary{
				{Version: "0.0.1", Status: "draft"},
			},
		}
		resp := V3Response{Code: 0, Message: "success"}
		data, _ := json.Marshal(detail)
		resp.Data = data
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	nacosClient, err := newTestNacosClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	d, err := NewPromptService(nacosClient).DescribePrompt("test-prompt")
	if err != nil {
		t.Fatal(err)
	}
	if d.EditingVersion != "0.0.1" {
		t.Fatalf("editingVersion = %s, want 0.0.1", d.EditingVersion)
	}
	if len(d.Versions) != 1 || d.Versions[0].Status != "draft" {
		t.Fatalf("unexpected versions: %+v", d.Versions)
	}
}

func TestGetPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v3/client/ai/prompt" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("promptKey"); got != "test-prompt" {
			t.Fatalf("promptKey = %s, want test-prompt", got)
		}
		if got := r.URL.Query().Get("version"); got != "1.0.0" {
			t.Fatalf("version = %s, want 1.0.0", got)
		}

		p := ClientPrompt{
			PromptKey: "test-prompt",
			Version:   "1.0.0",
			Template:  "Hello {{name}}!",
		}
		resp := V3Response{Code: 0, Message: "success"}
		data, _ := json.Marshal(p)
		resp.Data = data
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	nacosClient, err := newTestNacosClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	p, err := NewPromptService(nacosClient).GetPrompt("test-prompt", "1.0.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Template != "Hello {{name}}!" {
		t.Fatalf("template = %s, want Hello {{name}}!", p.Template)
	}
	if p.Version != "1.0.0" {
		t.Fatalf("version = %s, want 1.0.0", p.Version)
	}
}

func TestDraftCreatesNewPrompt(t *testing.T) {
	var createCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nacos/v3/admin/ai/prompt/governance":
			// Prompt doesn't exist yet -> return error
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 404, "message": "not found",
			})
		case "/nacos/v3/admin/ai/prompt/draft":
			if r.Method != http.MethodPost {
				t.Fatalf("draft method = %s, want POST", r.Method)
			}
			createCalled = true
			body, _ := io.ReadAll(r.Body)
			params := string(body)
			if !strings.Contains(params, "promptKey=test-prompt") {
				t.Fatalf("missing promptKey in body: %s", params)
			}
			if !strings.Contains(params, "template=") {
				t.Fatalf("missing template in body: %s", params)
			}
			resp := V3Response{Code: 0, Message: "success"}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	nacosClient, err := newTestNacosClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	err = NewPromptService(nacosClient).Draft("test-prompt", "Hello!", "", "init", "A test", "")
	if err != nil {
		t.Fatal(err)
	}
	if !createCalled {
		t.Fatal("createDraft was not called")
	}
}

func TestDraftUpdatesExistingDraft(t *testing.T) {
	var updateCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nacos/v3/admin/ai/prompt/governance":
			// Prompt exists with editing version
			detail := PromptDetail{
				PromptListItem: PromptListItem{
					PromptKey:      "test-prompt",
					EditingVersion: "0.0.1",
				},
			}
			resp := V3Response{Code: 0, Message: "success"}
			data, _ := json.Marshal(detail)
			resp.Data = data
			_ = json.NewEncoder(w).Encode(resp)
		case "/nacos/v3/admin/ai/prompt/draft":
			if r.Method != http.MethodPut {
				t.Fatalf("draft method = %s, want PUT", r.Method)
			}
			updateCalled = true
			resp := V3Response{Code: 0, Message: "success"}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	nacosClient, err := newTestNacosClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	err = NewPromptService(nacosClient).Draft("test-prompt", "Updated!", "", "update", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !updateCalled {
		t.Fatal("updateDraft was not called")
	}
}

func TestSubmitPrompt(t *testing.T) {
	var submitCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v3/admin/ai/prompt/submit" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		submitCalled = true
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		params := string(body)
		if !strings.Contains(params, "promptKey=test-prompt") {
			t.Fatalf("missing promptKey: %s", params)
		}
		if !strings.Contains(params, "namespaceId=test-ns") {
			t.Fatalf("missing namespaceId: %s", params)
		}
		resp := V3Response{Code: 0, Message: "success"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	nacosClient, err := newTestNacosClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	err = NewPromptService(nacosClient).Submit("test-prompt", "")
	if err != nil {
		t.Fatal(err)
	}
	if !submitCalled {
		t.Fatal("submit was not called")
	}
}

func TestPublishPrompt(t *testing.T) {
	var publishCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v3/admin/ai/prompt/publish" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		publishCalled = true
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		params := string(body)
		if !strings.Contains(params, "promptKey=test-prompt") {
			t.Fatalf("missing promptKey: %s", params)
		}
		if !strings.Contains(params, "version=1.0.0") {
			t.Fatalf("missing version: %s", params)
		}
		resp := V3Response{Code: 0, Message: "success"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	nacosClient, err := newTestNacosClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	err = NewPromptService(nacosClient).Publish("test-prompt", "1.0.0", true)
	if err != nil {
		t.Fatal(err)
	}
	if !publishCalled {
		t.Fatal("publish was not called")
	}
}

func TestPublishPromptNoUpdateLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		params := string(body)
		if !strings.Contains(params, "updateLatestLabel=false") {
			t.Fatalf("expected updateLatestLabel=false in body: %s", params)
		}
		resp := V3Response{Code: 0, Message: "success"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	nacosClient, err := newTestNacosClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	err = NewPromptService(nacosClient).Publish("test-prompt", "1.0.0", false)
	if err != nil {
		t.Fatal(err)
	}
}
