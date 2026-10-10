package chat

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestHumanizedToolName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "snake case", input: "mcp_github_get", want: "Mcp Github Get"},
		{name: "kebab case", input: "web-fetch", want: "Web Fetch"},
		{name: "mixed", input: "job_output-tool", want: "Job Output Tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := humanizedToolName(tt.input); got != tt.want {
				t.Fatalf("humanizedToolName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLooksLikeMarkdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "header", content: "# Title", want: true},
		{name: "code fence", content: "```go\nfmt.Println(\"x\")\n```", want: true},
		{name: "plain", content: "hello world", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := looksLikeMarkdown(tt.content); got != tt.want {
				t.Fatalf("looksLikeMarkdown() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderToolResultTextContent(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	styPtr := &sty
	widths := toolResultContentWidths{Body: 80, Diff: 82}

	t.Run("json branch", func(t *testing.T) {
		t.Parallel()
		content := `{"a":1}`
		var result json.RawMessage
		if err := json.Unmarshal([]byte(content), &result); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		prettyResult, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("json.MarshalIndent() error = %v", err)
		}
		expected := styPtr.Tool.Body.Render(toolOutputCodeContent(styPtr, "result.json", string(prettyResult), 0, widths.Body, false))
		got := renderToolResultTextContent(styPtr, content, widths, false)
		if got != expected {
			t.Fatal("renderToolResultTextContent() did not choose JSON rendering")
		}
	})

	t.Run("diff branch before markdown", func(t *testing.T) {
		t.Parallel()
		content := `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1 @@
-# Old
+# New
`
		expected := toolOutputDiffContentFromUnified(styPtr, content, widths.Diff, false)
		got := renderToolResultTextContent(styPtr, content, widths, false)
		if got != expected {
			t.Fatal("renderToolResultTextContent() did not choose diff rendering")
		}
	})

	t.Run("markdown branch", func(t *testing.T) {
		t.Parallel()
		content := "# Title\n\nBody"
		expected := styPtr.Tool.Body.Render(toolOutputCodeContent(styPtr, "result.md", content, 0, widths.Body, false))
		got := renderToolResultTextContent(styPtr, content, widths, false)
		if got != expected {
			t.Fatal("renderToolResultTextContent() did not choose markdown rendering")
		}
	})

	t.Run("plain branch", func(t *testing.T) {
		t.Parallel()
		content := "plain text"
		expected := styPtr.Tool.Body.Render(toolOutputPlainContent(styPtr, content, widths.Body, false))
		got := renderToolResultTextContent(styPtr, content, widths, false)
		if got != expected {
			t.Fatal("renderToolResultTextContent() did not choose plain rendering")
		}
	})
}

// hookMetaJSON builds a tool metadata string with the given hook sections.
func hookMetaJSON(pre, post *hooks.HookMetadata) string {
	type wrapper struct {
		Hook     *hooks.HookMetadata `json:"hook,omitempty"`
		PostHook *hooks.HookMetadata `json:"post_hook,omitempty"`
	}
	data, err := json.Marshal(wrapper{Hook: pre, PostHook: post})
	if err != nil {
		return ""
	}
	return string(data)
}

// TestToolOutputHookIndicatorPostHook pins the post-hook block: the
// PostToolUse metadata renders as a second section below the PreToolUse
// section, with the Hook [PostTU] label and the same alignment.
func TestToolOutputHookIndicatorPostHook(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	t.Run("both sections render", func(t *testing.T) {
		t.Parallel()
		pre := &hooks.HookMetadata{
			Event:     hooks.EventPreToolUse,
			HookCount: 1,
			Hooks:     []hooks.HookInfo{{Name: "rtk-rewrite.sh", Matcher: "^bash$", Decision: "allow"}},
		}
		post := &hooks.HookMetadata{
			Event:     hooks.EventPostToolUse,
			HookCount: 1,
			Hooks:     []hooks.HookInfo{{Name: "audit-tool.sh", Matcher: "^bash$", Decision: "allow"}},
		}
		out := ansi.Strip(toolOutputHookIndicator(&sty, hookMetaJSON(pre, post), 100, 0))
		require.Contains(t, out, "Hook [PreTU]")
		require.Contains(t, out, "rtk-rewrite.sh")
		require.Contains(t, out, "Hook [PostTU]")
		require.Contains(t, out, "audit-tool.sh")
		require.Contains(t, out, "\n\n", "one blank line separates the two blocks")
	})

	t.Run("post-only renders one section", func(t *testing.T) {
		t.Parallel()
		post := &hooks.HookMetadata{
			Event:     hooks.EventPostToolUse,
			HookCount: 1,
			Hooks:     []hooks.HookInfo{{Name: "audit-tool.sh", Decision: "allow"}},
		}
		out := ansi.Strip(toolOutputHookIndicator(&sty, hookMetaJSON(nil, post), 100, 0))
		require.Contains(t, out, "Hook [PostTU]")
		require.NotContains(t, out, "Hook [PreTU]")
	})

	t.Run("post deny styling applies", func(t *testing.T) {
		t.Parallel()
		post := &hooks.HookMetadata{
			Event:     hooks.EventPostToolUse,
			HookCount: 1,
			Hooks:     []hooks.HookInfo{{Name: "audit-tool.sh", Decision: "deny", Reason: "too late"}},
		}
		out := ansi.Strip(toolOutputHookIndicator(&sty, hookMetaJSON(nil, post), 100, 0))
		require.Contains(t, out, "Denied")
		require.Contains(t, out, "too late")
	})

	t.Run("empty metadata renders nothing", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, toolOutputHookIndicator(&sty, "", 100, 0))
		require.Empty(t, toolOutputHookIndicator(&sty, hookMetaJSON(nil, nil), 100, 0))
	})
}
