package hooks

import (
	"encoding/json"
	"testing"

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
