package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

func newRuntimeLocalSkillsListTestCmd() *cobra.Command {
	cmd := testCmd()
	cmd.Flags().String("output", "json", "")
	return cmd
}

func newRuntimeLocalSkillsImportTestCmd() *cobra.Command {
	cmd := testCmd()
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("output", "json", "")
	return cmd
}

func TestRunRuntimeLocalSkillsListPollsUntilCompleted(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/runtimes/runtime-1/local-skills":
			if r.Method != http.MethodPost {
				t.Fatalf("list initiate method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"id":"req-1","runtime_id":"runtime-1","status":"pending","supported":true}`))
		case "/api/runtimes/runtime-1/local-skills/req-1":
			if r.Method != http.MethodGet {
				t.Fatalf("list poll method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"id":"req-1","runtime_id":"runtime-1","status":"completed","supported":true,"skills":[{"key":"review","name":"Review","provider":"claude","source_path":"~/.claude/skills/review","file_count":2}]}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	t.Setenv("MULTICA_TOKEN", "test-token")

	cmd := newRuntimeLocalSkillsListTestCmd()
	if err := runRuntimeLocalSkillsList(cmd, []string{"runtime-1"}); err != nil {
		t.Fatalf("runRuntimeLocalSkillsList: %v", err)
	}
	want := []string{
		"POST /api/runtimes/runtime-1/local-skills",
		"GET /api/runtimes/runtime-1/local-skills/req-1",
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}

func TestRunRuntimeLocalSkillsImportPostsSkillKeyAndPolls(t *testing.T) {
	var gotBody map[string]any
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/runtimes/runtime-1/local-skills/import":
			if r.Method != http.MethodPost {
				t.Fatalf("import initiate method = %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"imp-1","runtime_id":"runtime-1","skill_key":"review","status":"running"}`))
		case "/api/runtimes/runtime-1/local-skills/import/imp-1":
			if r.Method != http.MethodGet {
				t.Fatalf("import poll method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"id":"imp-1","runtime_id":"runtime-1","skill_key":"review","status":"completed","skill":{"id":"skill-1","name":"Review"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	t.Setenv("MULTICA_TOKEN", "test-token")

	cmd := newRuntimeLocalSkillsImportTestCmd()
	_ = cmd.Flags().Set("name", "Override")
	if err := runRuntimeLocalSkillsImport(cmd, []string{"runtime-1", "review"}); err != nil {
		t.Fatalf("runRuntimeLocalSkillsImport: %v", err)
	}
	if gotBody["skill_key"] != "review" || gotBody["name"] != "Override" {
		t.Fatalf("import body = %#v", gotBody)
	}
	want := []string{
		"POST /api/runtimes/runtime-1/local-skills/import",
		"GET /api/runtimes/runtime-1/local-skills/import/imp-1",
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}
