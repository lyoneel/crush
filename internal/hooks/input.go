package hooks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/tidwall/gjson"
)

// SupportedOutputVersion is the highest envelope version this build
// understands. Hooks may omit `version` entirely (treated as 1) or pin
// an older version. Unknown higher versions are still parsed but logged.
const SupportedOutputVersion = 1

// SessionPayload carries the session snapshot data lifecycle hooks
// receive: the abandoned session on a switch, the newly active session,
// or the finished session. Counts are always present; a missing side of
// a transition marshals as JSON null.
type SessionPayload struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	MessageCount     int64   `json:"message_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
	UpdatedAt        int64   `json:"updated_at"`
}

// Payload is the JSON structure piped to hook commands via stdin.
// ToolInput is emitted as a parsed JSON object for compatibility with
// Claude Code hooks (which expect tool_input to be an object, not a
// string). The session fields only apply to lifecycle events: a tool
// event leaves them unset.
type Payload struct {
	Event     string          `json:"event"`
	SessionID string          `json:"session_id"`
	CWD       string          `json:"cwd"`
	ToolName  string          `json:"tool_name,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	Prompt    string          `json:"prompt,omitempty"`

	// Lifecycle events only. A nil side marshals as null, so a session
	// switch away from a first session carries previous_session: null.
	PreviousSession *SessionPayload `json:"previous_session"`
	NewSession      *SessionPayload `json:"new_session"`
	// AgentFinished carries the finished session here.
	Session *SessionPayload `json:"session,omitempty"`
}

// BuildPayload constructs the JSON stdin payload for a hook command.
func BuildPayload(eventName, sessionID, cwd, toolName, toolInputJSON string) []byte {
	toolInput := json.RawMessage(toolInputJSON)
	if !json.Valid(toolInput) {
		toolInput = json.RawMessage("{}")
	}
	p := Payload{
		Event:     eventName,
		SessionID: sessionID,
		CWD:       cwd,
		ToolName:  toolName,
		ToolInput: toolInput,
	}
	data, err := json.Marshal(p)
	if err != nil {
		return []byte("{}")
	}
	return data
}

// SessionEventData carries the lifecycle-specific payload fields.
// Previous and New apply to SessionSwitch; Prompt applies to
// PromptSubmitted; Session applies to NewSession, SessionStart, and
// AgentFinished.
type SessionEventData struct {
	Previous *SessionPayload
	New      *SessionPayload
	Session  *SessionPayload
	Prompt   string
}

// BuildSessionEventPayload constructs the JSON stdin payload for a
// lifecycle hook command. Tool fields stay unset.
func BuildSessionEventPayload(eventName, sessionID, cwd string, data SessionEventData) []byte {
	p := Payload{
		Event:           eventName,
		SessionID:       sessionID,
		CWD:             cwd,
		PreviousSession: data.Previous,
		NewSession:      data.New,
		Session:         data.Session,
		Prompt:          data.Prompt,
	}
	out, err := json.Marshal(p)
	if err != nil {
		return []byte("{}")
	}
	return out
}

// BuildEnv constructs the environment variable slice for a hook command.
// It includes all current process env vars plus hook-specific ones.
// previousSessionID and newSessionID apply to lifecycle events; tool
// events pass empty strings, which yield empty variables.
func BuildEnv(eventName, toolName, sessionID, cwd, projectDir, toolInputJSON, previousSessionID, newSessionID string) []string {
	env := os.Environ()
	env = append(env, shell.CrushEnvMarkers()...)
	env = append(
		env,
		fmt.Sprintf("CRUSH_EVENT=%s", eventName),
		fmt.Sprintf("CRUSH_TOOL_NAME=%s", toolName),
		fmt.Sprintf("CRUSH_SESSION_ID=%s", sessionID),
		fmt.Sprintf("CRUSH_CWD=%s", cwd),
		fmt.Sprintf("CRUSH_PROJECT_DIR=%s", projectDir),
		fmt.Sprintf("CRUSH_PREVIOUS_SESSION_ID=%s", previousSessionID),
		fmt.Sprintf("CRUSH_NEW_SESSION_ID=%s", newSessionID),
	)

	// Extract tool-specific env vars from the JSON input.
	if toolInputJSON != "" {
		if cmd := gjson.Get(toolInputJSON, "command"); cmd.Exists() {
			env = append(env, fmt.Sprintf("CRUSH_TOOL_INPUT_COMMAND=%s", cmd.String()))
		}
		if fp := gjson.Get(toolInputJSON, "file_path"); fp.Exists() {
			env = append(env, fmt.Sprintf("CRUSH_TOOL_INPUT_FILE_PATH=%s", fp.String()))
		}
	}

	return env
}

// parseStdout parses the JSON output from a hook command's stdout.
// Supports both Crush format and Claude Code format (hookSpecificOutput).
func parseStdout(stdout string) HookResult {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return HookResult{Decision: DecisionNone}
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return HookResult{Decision: DecisionNone}
	}

	// Claude Code compat: if hookSpecificOutput is present, parse that.
	if hso, ok := raw["hookSpecificOutput"]; ok {
		return parseClaudeCodeOutput(hso)
	}

	var parsed struct {
		Version      int             `json:"version"`
		Decision     string          `json:"decision"`
		Halt         bool            `json:"halt"`
		Reason       string          `json:"reason"`
		Context      json.RawMessage `json:"context"`
		UpdatedInput json.RawMessage `json:"updated_input"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		return HookResult{Decision: DecisionNone}
	}

	if parsed.Version > SupportedOutputVersion {
		slog.Debug(
			"Hook output declared a newer envelope version than this build supports",
			"version", parsed.Version,
			"supported", SupportedOutputVersion,
		)
	}

	result := HookResult{
		Halt:    parsed.Halt,
		Reason:  parsed.Reason,
		Context: parseContext(parsed.Context),
	}
	result.Decision = parseDecision(parsed.Decision)
	result.UpdatedInput = rawToString(parsed.UpdatedInput)
	return result
}

// parseContext accepts either a single string or an array of strings and
// returns a newline-joined value with empty entries dropped.
func parseContext(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// String form.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return ""
	}
	// Array form.
	if raw[0] == '[' {
		var items []string
		if err := json.Unmarshal(raw, &items); err != nil {
			return ""
		}
		out := items[:0]
		for _, s := range items {
			if s != "" {
				out = append(out, s)
			}
		}
		return strings.Join(out, "\n")
	}
	return ""
}

// parseClaudeCodeOutput handles the Claude Code hook output format:
// {"hookSpecificOutput": {"permissionDecision": "allow", ...}}
func parseClaudeCodeOutput(data json.RawMessage) HookResult {
	var hso struct {
		PermissionDecision       string          `json:"permissionDecision"`
		PermissionDecisionReason string          `json:"permissionDecisionReason"`
		UpdatedInput             json.RawMessage `json:"updatedInput"`
		AdditionalContext        string          `json:"additionalContext"`
	}
	if err := json.Unmarshal(data, &hso); err != nil {
		return HookResult{Decision: DecisionNone}
	}

	result := HookResult{
		Decision: parseDecision(hso.PermissionDecision),
		Reason:   hso.PermissionDecisionReason,
		Context:  hso.AdditionalContext,
	}

	// Marshal updatedInput back to a string for our opaque format.
	if len(hso.UpdatedInput) > 0 && string(hso.UpdatedInput) != "null" {
		result.UpdatedInput = string(hso.UpdatedInput)
	}

	return result
}

// rawToString converts a json.RawMessage to a string suitable for use
// as opaque tool input. It accepts both a JSON object (nested) and a
// JSON string (stringified, for backward compatibility).
func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// If it's a JSON string, unwrap it.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	// Otherwise it's an object/array — use as-is.
	return string(raw)
}

func parseDecision(s string) Decision {
	switch strings.ToLower(s) {
	case "allow":
		return DecisionAllow
	case "deny":
		return DecisionDeny
	default:
		return DecisionNone
	}
}
