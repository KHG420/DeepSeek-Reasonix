package agent_test

import (
	"context"
	"reasonix/internal/contract/event"
	"reasonix/internal/contract/eventwire"
	"reasonix/internal/contract/provider"
	"reasonix/internal/contract/tool"
	"reasonix/internal/ext/skill"
	"reasonix/internal/runtime/agent"
	"reasonix/internal/state/sessionstore"
	"strings"
	"sync"
	"testing"
)

type exploreCallProvider struct {
	args string
	sent bool
}

func (*exploreCallProvider) Name() string { return "explore-call" }

func (p *exploreCallProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 2)
	if !p.sent {
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "explore-1", Name: "explore", Arguments: p.args}}
		p.sent = true
	} else {
		ch <- provider.Chunk{Type: provider.ChunkText, Text: "The investigation call has returned."}
	}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func TestExploreArgumentRefusalReachesStudioAndStoredSession(t *testing.T) {
	for _, tc := range []struct {
		name, args, hint, code string
		wantRuns               int
		runnerErr              error
	}{
		{name: "missing task", args: `{}`, hint: `it requires "task"`, code: "tool.arguments_invalid"},
		{name: "mistyped task", args: `{"task":7}`, hint: `"task" must be a string, not a number`, code: "tool.arguments_invalid"},
		{name: "valid task", args: `{"task":"Find the queue editor"}`, wantRuns: 1},
		{name: "cancelled runner", args: `{"task":"Find the queue editor"}`, hint: "context canceled", wantRuns: 1, runnerErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store := skill.New(skill.Options{HomeDir: root, ProjectRoot: root})
			runs := 0
			reg := tool.NewRegistry()
			for _, tl := range skill.BuiltinSubagentTools(store, func(context.Context, skill.Skill, string, skill.SubagentRunOptions) (string, error) {
				runs++
				return "Found the queue editor", tc.runnerErr
			}) {
				if tl.Name() == "explore" {
					reg.Add(tl)
				}
			}
			sess := sessionstore.NewSession(root)
			var mu sync.Mutex
			var dispatch event.Event
			var result event.Event
			sink := event.FuncSink(func(e event.Event) {
				if e.Tool.ID == "explore-1" {
					mu.Lock()
					switch e.Kind {
					case event.ToolDispatch:
						dispatch = e
					case event.ToolResult:
						result = e
					}
					mu.Unlock()
				}
			})
			a := agent.New(&exploreCallProvider{args: tc.args}, reg, sess, agent.Options{}, sink)
			if err := a.Run(context.Background(), "Investigate where the queue editor lives"); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if runs != tc.wantRuns {
				t.Fatalf("subagent runner called %d times, want %d", runs, tc.wantRuns)
			}
			wire := eventwire.ToWire(result)
			start := eventwire.ToWire(dispatch)
			if start.Tool == nil || start.Tool.Profile == nil || start.Tool.Profile.Name == "" || wire.Tool == nil {
				t.Fatalf("Studio cannot identify the explore row: dispatch=%+v result=%+v", start.Tool, wire.Tool)
			}
			if wire.Tool.RefusalCode != tc.code {
				t.Errorf("Studio refusal identity = %q, want %q", wire.Tool.RefusalCode, tc.code)
			}
			if tc.hint != "" && !strings.Contains(wire.Tool.Err, tc.hint) {
				t.Errorf("Studio error = %q, want %q", wire.Tool.Err, tc.hint)
			}
			var stored *provider.Message
			for i := range sess.Messages {
				if msg := &sess.Messages[i]; msg.Role == provider.RoleTool && msg.ToolCallID == "explore-1" {
					stored = msg
				}
			}
			if stored == nil {
				t.Fatal("explore result missing from stored session")
			}
			if tc.hint == "" {
				if wire.Tool.Err != "" || stored.ToolFailure != nil {
					t.Fatalf("successful runner recorded as a failure: wire=%+v stored=%+v", wire.Tool, stored)
				}
			} else if stored.ToolFailure == nil || stored.ToolFailure.RefusalCode != tc.code {
				t.Errorf("stored refusal = %+v, want code %q", stored.ToolFailure, tc.code)
			}
		})
	}
}
