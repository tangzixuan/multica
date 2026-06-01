package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func newSkillCreateBundleTestCmd() *cobra.Command {
	cmd := testCmd()
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("content", "", "")
	cmd.Flags().String("config", "", "")
	cmd.Flags().String("bundle-dir", "", "")
	cmd.Flags().String("output", "json", "")
	return cmd
}

func newSkillUpdateBundleTestCmd() *cobra.Command {
	cmd := testCmd()
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("content", "", "")
	cmd.Flags().String("config", "", "")
	cmd.Flags().String("bundle-dir", "", "")
	cmd.Flags().String("output", "json", "")
	return cmd
}

func writeSkillBundle(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

func TestRunSkillCreateBundleDirPostsSkillWithFiles(t *testing.T) {
	bundleDir := writeSkillBundle(t, map[string]string{
		"SKILL.md":            "---\nname: Bundle Helper\ndescription: Ships as a directory\n---\n# Bundle Helper\n",
		"references/api.md":   "api notes",
		"templates/prompt.md": "prompt body",
		"scripts/run.sh":      "#!/bin/sh\ntrue\n",
		"LICENSE":             "ignored",
		".hidden/secret.txt":  "ignored",
	})

	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"skill-1","name":"Bundle Helper","files":[]}`))
	}))
	defer srv.Close()

	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	t.Setenv("MULTICA_TOKEN", "test-token")

	cmd := newSkillCreateBundleTestCmd()
	_ = cmd.Flags().Set("bundle-dir", bundleDir)
	if err := runSkillCreate(cmd, nil); err != nil {
		t.Fatalf("runSkillCreate: %v", err)
	}

	if gotMethod != http.MethodPost || gotPath != "/api/skills" {
		t.Fatalf("request = %s %s, want POST /api/skills", gotMethod, gotPath)
	}
	if gotBody["name"] != "Bundle Helper" || gotBody["description"] != "Ships as a directory" {
		t.Fatalf("frontmatter fields not mapped: %#v", gotBody)
	}
	if gotBody["content"] != "---\nname: Bundle Helper\ndescription: Ships as a directory\n---\n# Bundle Helper\n" {
		t.Fatalf("content = %#v", gotBody["content"])
	}
	files := gotBody["files"].([]any)
	gotFiles := make(map[string]string, len(files))
	for _, item := range files {
		f := item.(map[string]any)
		gotFiles[f["path"].(string)] = f["content"].(string)
	}
	wantFiles := map[string]string{
		"references/api.md":   "api notes",
		"scripts/run.sh":      "#!/bin/sh\ntrue\n",
		"templates/prompt.md": "prompt body",
	}
	if !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Fatalf("files = %#v, want %#v", gotFiles, wantFiles)
	}
}

func TestRunSkillUpdateBundleDirReplacesFiles(t *testing.T) {
	bundleDir := writeSkillBundle(t, map[string]string{
		"SKILL.md":           "---\nname: Updated Helper\ndescription: Updated desc\n---\n# Updated\n",
		"assets/example.txt": "asset",
	})

	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"skill-123","name":"Updated Helper","files":[]}`))
	}))
	defer srv.Close()

	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	t.Setenv("MULTICA_TOKEN", "test-token")

	cmd := newSkillUpdateBundleTestCmd()
	_ = cmd.Flags().Set("bundle-dir", bundleDir)
	if err := runSkillUpdate(cmd, []string{"skill-123"}); err != nil {
		t.Fatalf("runSkillUpdate: %v", err)
	}

	if gotMethod != http.MethodPut || gotPath != "/api/skills/skill-123" {
		t.Fatalf("request = %s %s, want PUT /api/skills/skill-123", gotMethod, gotPath)
	}
	if gotBody["name"] != "Updated Helper" || gotBody["description"] != "Updated desc" {
		t.Fatalf("frontmatter fields not mapped: %#v", gotBody)
	}
	files := gotBody["files"].([]any)
	if len(files) != 1 || files[0].(map[string]any)["path"] != "assets/example.txt" {
		t.Fatalf("files = %#v, want replacement bundle files", files)
	}
}
