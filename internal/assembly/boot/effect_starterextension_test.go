package boot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
	"reasonix/internal/ext/installsource"
	"reasonix/internal/ext/pluginpkg"
	"reasonix/internal/session/control"
)

func TestEffectStarterExtensionComposedInput(t *testing.T) {
	example, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk", "go", "examples", "starterextension"))
	if err != nil {
		t.Fatal(err)
	}
	source := robustTempDir(t)
	binary := filepath.Join(source, "bin", "starter-extension.exe")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	buildCtx, cancelBuild := context.WithTimeout(t.Context(), time.Minute)
	defer cancelBuild()
	cmd := exec.CommandContext(buildCtx, "go", "build", "-o", binary, ".")
	cmd.Dir = example
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build starter extension: %v\n%s", err, out)
	}
	manifest, err := os.ReadFile(filepath.Join(example, pluginpkg.NativeManifest))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, pluginpkg.NativeManifest), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	home := isolateConfigHome(t)
	reasonixHome := filepath.Join(home, ".reasonix")
	t.Setenv("REASONIX_HOME", reasonixHome)
	workspace := robustTempDir(t)
	t.Chdir(workspace)
	providerKind := "boot-effect-starter-" + filepath.Base(workspace)
	writeFile(t, workspace, "reasonix.toml", fmt.Sprintf(`
default_model = "test-model"

[agent]
system_prompt = "STARTER BASE"

[environment]
enabled = false

[codegraph]
enabled = false

[[providers]]
name = "test-model"
kind = %q
model = "x"
`, providerKind))
	approveWorkspace(t, workspace)
	installer := installsource.NewTool(installsource.Options{ProjectRoot: workspace, HomeDir: home, RequireApprovedPlan: true})
	install := func(args map[string]any) string {
		t.Helper()
		raw, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		out, err := installer.Execute(t.Context(), raw)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			OK      bool   `json:"ok"`
			Applied bool   `json:"applied"`
			Status  string `json:"status"`
			PlanID  string `json:"planId"`
		}
		if err := json.Unmarshal([]byte(out), &result); err != nil || !result.OK {
			t.Fatalf("install_source = %s, err=%v", out, err)
		}
		if args["apply"] == true || args["op"] == "uninstall" {
			if !result.Applied || result.Status != "done" {
				t.Fatalf("install_source did not apply: %s", out)
			}
		} else if result.Applied || result.Status != "planned" || result.PlanID == "" {
			t.Fatalf("install_source did not preview: %s", out)
		}
		return result.PlanID
	}
	rec := &effectRecordingProvider{}
	provider.Register(providerKind, func(provider.Config) (provider.Provider, error) { return rec, nil })
	baseline := map[string]string{}
	runPhase := func(t *testing.T, enabled bool) {
		t.Helper()
		res, err := BuildRuntime(t.Context(), Options{Sink: event.Discard})
		if err != nil {
			t.Fatal(err)
		}
		defer res.Controller.Close()
		if enabled {
			if res.Extensions == nil || res.Extensions.Client("starter-extension") == nil {
				t.Fatal("installed starter extension did not start")
			}
			client := res.Extensions.Client("starter-extension")
			defer func() {
				res.Controller.Close()
				waitForCond(t, "starter extension process exit", 10*time.Second, client.Exited)
			}()
		} else if res.Extensions != nil {
			t.Fatal("inactive starter extension retained a runtime")
		}
		res.Controller.SetResponseLanguage("zh")
		res.Controller.EnsureSessionPath()
		wantSuffix := 0
		if enabled {
			wantSuffix = 1
		}
		for _, input := range []string{"starter: explain sidecars", "explain sidecars"} {
			before := len(rec.requests())
			res.Controller.SubmitHTTPOptions(input, control.SubmitOptions{RefuseUnknownSlash: true})
			waitForCond(t, "starter provider request", 10*time.Second, func() bool { return len(rec.requests()) > before })
			waitForCond(t, "starter turn completion", 10*time.Second, func() bool { return !res.Controller.Running() })
			reqs := agentRequests(rec.requests()[before:])
			if len(reqs) == 0 {
				t.Fatal("starter turn never reached the recording provider")
			}
			text := lastUserOf(reqs[0])
			const suffix = " [rewritten by starter-extension]"
			if !strings.Contains(text, "<response-language>") || !strings.Contains(text, input) || strings.Count(text, suffix) != wantSuffix {
				t.Fatalf("enabled=%t, input=%q, provider user text = %q", enabled, input, text)
			}
			if _, ok := baseline[input]; !ok {
				baseline[input] = text
			} else if strings.Replace(text, suffix, "", 1) != baseline[input] {
				t.Fatalf("starter extension changed the composed input beyond its marker: %q", text)
			}
			if !strings.Contains(systemText(reqs[0]), "STARTER BASE") || strings.Contains(systemText(reqs[0]), suffix) {
				t.Fatal("starter rewrite changed the system prompt")
			}
		}
	}
	t.Run("absent", func(t *testing.T) { runPhase(t, false) })
	args := map[string]any{"source": source, "kind": "plugin", "mode": "copy", "scope": "global"}
	args["planId"] = install(args)
	args["apply"] = true
	install(args)
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	t.Run("installed", func(t *testing.T) { runPhase(t, true) })
	if err := pluginpkg.SetEnabled(reasonixHome, "starter-extension", false); err != nil {
		t.Fatal(err)
	}
	t.Run("disabled", func(t *testing.T) { runPhase(t, false) })
	if err := pluginpkg.SetEnabled(reasonixHome, "starter-extension", true); err != nil {
		t.Fatal(err)
	}
	t.Run("reenabled", func(t *testing.T) { runPhase(t, true) })
	install(map[string]any{"op": "uninstall", "name": "starter-extension", "kind": "plugin", "scope": "global", "apply": true})
	t.Run("removed", func(t *testing.T) { runPhase(t, false) })
}
