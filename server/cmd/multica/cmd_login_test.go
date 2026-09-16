package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

func newLoginTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "login"}
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("profile", "", "")
	return cmd
}

func TestResolveLoginTokenServerURLDefaultsToCloud(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MULTICA_SERVER_URL", "")

	if got := resolveLoginTokenServerURL(newLoginTestCmd()); got != defaultCloudServerURL {
		t.Fatalf("resolveLoginTokenServerURL() = %q, want %q", got, defaultCloudServerURL)
	}
}

func TestResolveLoginTokenServerURLPrefersConfiguredServer(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MULTICA_SERVER_URL", "")
	// A stale host/container value is not proof that this process is running
	// inside a daemon task. Login still needs the selected human profile.
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	t.Setenv(cli.TaskConfigRootEnv, "")
	t.Setenv("MULTICA_DAEMON_PORT", "20032")
	cmd := newLoginTestCmd()
	if err := cmd.Flags().Set("profile", "jcode"); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{ServerURL: "https://api.example.test/"}, "jcode"); err != nil {
		t.Fatalf("SaveCLIConfig: %v", err)
	}

	if got := resolveLoginTokenServerURL(cmd); got != "https://api.example.test" {
		t.Fatalf("resolveLoginTokenServerURL() = %q, want configured server", got)
	}
}

func TestRunLoginTokenAutoWatchesDiscoveredWorkspaces(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MULTICA_TOKEN", "")
	t.Setenv("MULTICA_WORKSPACE_ID", "")
	// Regression for #6779: older container setups may leave this daemon-
	// injected task hint in the host environment. It must not block the
	// explicitly human login flow or hide the profile just written by it.
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	t.Setenv(cli.TaskConfigRootEnv, "")
	t.Setenv("MULTICA_DAEMON_PORT", "20032")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mul_test_token" {
			t.Fatalf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/me":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name":  "Ada",
				"email": "ada@example.com",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/workspaces":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "ws-1", "name": "Alpha"},
				{"id": "ws-2", "name": "Beta"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)

	cmd := newLoginTestCmd()
	if err := cmd.Flags().Set("token", "mul_test_token"); err != nil {
		t.Fatalf("set token: %v", err)
	}
	if err := cmd.Flags().Set("profile", "jcode"); err != nil {
		t.Fatalf("set profile: %v", err)
	}

	stderr := captureStderr(t)
	err := runLogin(cmd, nil)
	errOut := stderr.read()
	if err != nil {
		t.Fatalf("runLogin: %v", err)
	}
	if !strings.Contains(errOut, "Found 2 workspace(s):") || !strings.Contains(errOut, "daemon start") {
		t.Fatalf("stderr = %q, want workspace discovery and daemon hint", errOut)
	}

	cfg, err := cli.LoadCLIConfigForProfile("jcode")
	if err != nil {
		t.Fatalf("LoadCLIConfig: %v", err)
	}
	if cfg.Token != "mul_test_token" || cfg.ServerURL != srv.URL || cfg.WorkspaceID != "ws-1" {
		t.Fatalf("config = %#v, want token, server URL, and first workspace", cfg)
	}
}

func TestWorkspaceCommandHasSwitchNotWatch(t *testing.T) {
	found, _, err := workspaceCmd.Find([]string{"switch"})
	if err != nil || found == nil || found.Name() != "switch" {
		t.Fatalf("workspace switch missing: found=%v err=%v", found, err)
	}
	if watch, _, watchErr := workspaceCmd.Find([]string{"watch"}); watchErr == nil && watch != nil && watch != workspaceCmd {
		t.Fatalf("workspace watch must not exist; found %q", watch.Use)
	}
}

func TestRunLoginWorkspaceSetupFailureHintsSwitch(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MULTICA_TOKEN", "")
	t.Setenv("MULTICA_WORKSPACE_ID", "")
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	t.Setenv(cli.TaskConfigRootEnv, "")
	t.Setenv("MULTICA_DAEMON_PORT", "20032")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mul_test_token" {
			t.Fatalf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/me":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name":  "Ada",
				"email": "ada@example.com",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/workspaces":
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "boom"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)

	cmd := newLoginTestCmd()
	if err := cmd.Flags().Set("token", "mul_test_token"); err != nil {
		t.Fatalf("set token: %v", err)
	}
	if err := cmd.Flags().Set("profile", "jcode"); err != nil {
		t.Fatalf("set profile: %v", err)
	}

	stderr := captureStderr(t)
	err := runLogin(cmd, nil)
	errOut := stderr.read()
	if err != nil {
		t.Fatalf("runLogin: %v", err)
	}
	if !strings.Contains(errOut, "Could not auto-configure workspaces") {
		t.Fatalf("stderr = %q, want auto-configure failure", errOut)
	}
	if !strings.Contains(errOut, "multica workspace list") || !strings.Contains(errOut, "multica workspace switch <id|slug>") {
		t.Fatalf("stderr = %q, want list + switch next steps", errOut)
	}
	if strings.Contains(errOut, "workspace watch") {
		t.Fatalf("stderr = %q, must not recommend removed workspace watch", errOut)
	}
	if strings.Contains(errOut, "daemon start") {
		t.Fatalf("stderr = %q, must not print the success daemon hint", errOut)
	}

	cfg, err := cli.LoadCLIConfigForProfile("jcode")
	if err != nil {
		t.Fatalf("LoadCLIConfig: %v", err)
	}
	if cfg.Token != "mul_test_token" || cfg.ServerURL != srv.URL {
		t.Fatalf("config = %#v, want saved login even when workspace setup fails", cfg)
	}
}
