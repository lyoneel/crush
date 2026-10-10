package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestBuildSessionEventPayload_Switch pins the PreSessionSwitch payload:
// both sides populated, counts always present, tool fields unset.
func TestBuildSessionEventPayload_Switch(t *testing.T) {
	payload := BuildSessionEventPayload(EventPreSessionSwitch, "sess-2", "/work", SessionEventData{
		Previous: &SessionPayload{ID: "sess-1", Title: "Old", PromptTokens: 10, CompletionTokens: 5, Cost: 0.5, UpdatedAt: 123},
		New:      &SessionPayload{ID: "sess-2", Title: "New Session"},
	})

	var p Payload
	require.NoError(t, json.Unmarshal(payload, &p))
	require.Equal(t, EventPreSessionSwitch, p.Event)
	require.Equal(t, "sess-2", p.SessionID)
	require.Equal(t, "/work", p.CWD)
	require.NotNil(t, p.PreviousSession)
	require.Equal(t, "sess-1", p.PreviousSession.ID)
	require.Equal(t, "Old", p.PreviousSession.Title)
	require.Equal(t, int64(10), p.PreviousSession.PromptTokens)
	require.NotNil(t, p.NewSession)
	require.Equal(t, "sess-2", p.NewSession.ID)
	require.Empty(t, p.ToolName)
	require.Nil(t, p.ToolInput)
	require.Empty(t, p.Prompt)
}

// TestBuildSessionEventPayload_FirstSession pins the first-session
// switch: the abandoned side marshals as JSON null.
func TestBuildSessionEventPayload_FirstSession(t *testing.T) {
	payload := BuildSessionEventPayload(EventPreSessionSwitch, "sess-1", "/work", SessionEventData{
		New: &SessionPayload{ID: "sess-1"},
	})

	var p Payload
	require.NoError(t, json.Unmarshal(payload, &p))
	require.Nil(t, p.PreviousSession)
	require.NotNil(t, p.NewSession)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(payload, &raw))
	require.Contains(t, raw, "previous_session")
	require.Nil(t, raw["previous_session"])
}

// TestBuildSessionEventPayload_PrePrompt pins the prompt payload.
func TestBuildSessionEventPayload_PrePrompt(t *testing.T) {
	payload := BuildSessionEventPayload(EventPrePrompt, "sess-1", "/work", SessionEventData{
		Prompt: "fix the bug",
	})

	var p Payload
	require.NoError(t, json.Unmarshal(payload, &p))
	require.Equal(t, EventPrePrompt, p.Event)
	require.Equal(t, "fix the bug", p.Prompt)
	require.Nil(t, p.PreviousSession)
	require.Nil(t, p.NewSession)
}

// TestBuildSessionEventPayload_PostResponse pins the response payload:
// the final assistant text serializes under response.
func TestBuildSessionEventPayload_PostResponse(t *testing.T) {
	payload := BuildSessionEventPayload(EventPostResponse, "sess-1", "/work", SessionEventData{
		Response: "all done",
	})

	var p Payload
	require.NoError(t, json.Unmarshal(payload, &p))
	require.Equal(t, EventPostResponse, p.Event)
	require.Equal(t, "all done", p.Response)
}

// TestBuildLifecycleEnv pins the lifecycle env vars, including the
// empty-variable shape when a side is absent.
func TestBuildLifecycleEnv(t *testing.T) {
	env := BuildEnv(EventPreSessionSwitch, "", "sess-2", "/work", "/project", "", "sess-1", "sess-2")
	joined := ""
	for i, v := range env {
		if i > 0 {
			joined += "\n"
		}
		joined += v
	}
	require.Contains(t, joined, "CRUSH_EVENT=PreSessionSwitch")
	require.Contains(t, joined, "CRUSH_PREVIOUS_SESSION_ID=sess-1")
	require.Contains(t, joined, "CRUSH_NEW_SESSION_ID=sess-2")

	absent := BuildEnv(EventPostResponse, "", "sess-1", "/work", "/project", "", "", "")
	joined = ""
	for i, v := range absent {
		if i > 0 {
			joined += "\n"
		}
		joined += v
	}
	require.Contains(t, joined, "CRUSH_PREVIOUS_SESSION_ID=")
	require.Contains(t, joined, "CRUSH_NEW_SESSION_ID=")
}

// TestBuildPayloadWithUsage pins the tool-event payload with a model
// context snapshot: the fields serialize under context_usage, the nil
// form omits the whole field.
func TestBuildPayloadWithUsage(t *testing.T) {
	payload := BuildPayloadWithUsage(EventPostToolUse, "sess-1", "/work", "bash", `{}`, "out", &ContextUsage{
		ModelID:        "glm-5.3-flash",
		ContextWindow:  200000,
		EstimatedUsage: true,
	})
	var p Payload
	require.NoError(t, json.Unmarshal(payload, &p))
	require.NotNil(t, p.ContextUsage)
	require.Equal(t, "glm-5.3-flash", p.ContextUsage.ModelID)
	require.Equal(t, int64(200000), p.ContextUsage.ContextWindow)
	require.True(t, p.ContextUsage.EstimatedUsage)

	absent := BuildPayloadWithUsage(EventPostToolUse, "sess-1", "/work", "bash", `{}`, "out", nil)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(absent, &raw))
	require.NotContains(t, raw, "context_usage")
}

// TestBuildLifecyclePayloadWithUsage pins the lifecycle payload with a
// model context snapshot riding next to the session snapshot.
func TestBuildLifecyclePayloadWithUsage(t *testing.T) {
	payload := BuildSessionEventPayloadWithUsage(EventPostPrompt, "sess-1", "/work",
		SessionEventData{Prompt: "go", Session: &SessionPayload{ID: "sess-1"}},
		&ContextUsage{ModelID: "m-1", ContextWindow: 128000},
	)
	var p Payload
	require.NoError(t, json.Unmarshal(payload, &p))
	require.Equal(t, "m-1", p.ContextUsage.ModelID)
	require.Equal(t, int64(128000), p.ContextUsage.ContextWindow)
	require.False(t, p.ContextUsage.EstimatedUsage)
	require.NotNil(t, p.Session)
}

// TestBuildContextUsageZeroOmits pins the zero-source conversion: an
// unknown model and window serialize with all optional fields empty.
func TestBuildContextUsageZeroOmits(t *testing.T) {
	usage := BuildContextUsage(UsageSource{})
	require.NotNil(t, usage)
	data, err := json.Marshal(usage)
	require.NoError(t, err)
	require.Equal(t, "{}", string(data))
}

// TestAppendUsageEnv pins the env wiring: nil emits nothing, a
// populated snapshot emits the three CRUSH_ variables.
func TestAppendUsageEnv(t *testing.T) {
	require.Nil(t, AppendUsageEnv(nil, nil))

	env := AppendUsageEnv([]string{}, &ContextUsage{
		ModelID:        "m-1",
		ContextWindow:  128000,
		EstimatedUsage: true,
	})
	joined := strings.Join(env, "\n")
	require.Contains(t, joined, "CRUSH_MODEL_ID=m-1")
	require.Contains(t, joined, "CRUSH_CONTEXT_WINDOW=128000")
	require.Contains(t, joined, "CRUSH_ESTIMATED_USAGE=true")
}

// TestRunnerUsageSource pins the runner wiring: a source attached to a
// runner lands in every tool-event payload; a zero result omits the
// field. Lifecycle payload coverage lives with the payload builder
// tests above.
func TestRunnerUsageSource(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "out.json")
	hookCfg := config.HookConfig{Command: "cat > " + logFile}
	r := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	r.SetUsageSource(func(string) UsageSource {
		return UsageSource{
			SessionPromptTokens:     100,
			SessionCompletionTokens: 50,
			SessionEstimatedUsage:   true,
			ModelID:                 "m-1",
			ModelContextWindow:      200000,
		}
	})
	_, err := r.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`, "")
	require.NoError(t, err)
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), `"context_usage":{"model_id":"m-1","context_window":200000,"estimated_usage":true}`)

	r2 := NewRunner([]config.HookConfig{hookCfg}, t.TempDir(), t.TempDir())
	r2.SetUsageSource(func(string) UsageSource { return UsageSource{} })
	_, err = r2.Run(context.Background(), EventPreToolUse, "sess", "bash", `{}`, "")
	require.NoError(t, err)
	data, err = os.ReadFile(logFile)
	require.NoError(t, err)
	require.NotContains(t, string(data), "context_usage")
}
