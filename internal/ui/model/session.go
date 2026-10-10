package model

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/x/ansi"
)

// loadSessionMsg is a message indicating that a session and its files have
// been loaded.
type loadSessionMsg struct {
	session   *session.Session
	files     []SessionFile
	readFiles []string
	messages  []message.Message
}

// lspFilePaths returns deduplicated file paths from both modified and read
// files for starting LSP servers.
func (msg loadSessionMsg) lspFilePaths() []string {
	seen := make(map[string]struct{}, len(msg.files)+len(msg.readFiles))
	paths := make([]string, 0, len(msg.files)+len(msg.readFiles))
	for _, f := range msg.files {
		p := f.LatestVersion.Path
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	for _, p := range msg.readFiles {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	return paths
}

// SessionFile tracks the first and latest versions of a file in a session,
// along with the total additions and deletions.
type SessionFile struct {
	FirstVersion  history.File
	LatestVersion history.File
	Additions     int
	Deletions     int
}

// loadSession loads the session along with its associated files and computes
// the diff statistics (additions and deletions) for each file in the session.
// It returns a tea.Cmd that, when executed, fetches the session data and
// returns a sessionFilesLoadedMsg containing the processed session files.
//
// The returned batch also reports the new current-session selection to
// the workspace so the server can update its per-client presence map.
// That report is fire-and-forget: errors are logged at debug and the
// UI never blocks on the call.
func (m *UI) loadSession(sessionID string) tea.Cmd {
	load := func() tea.Msg {
		session, err := m.com.Workspace.GetSession(context.Background(), sessionID)
		if err != nil {
			return util.ReportError(err)
		}

		sessionFiles, err := m.loadSessionFiles(sessionID)
		if err != nil {
			return util.ReportError(err)
		}

		readFiles, err := m.com.Workspace.FileTrackerListReadFiles(context.Background(), sessionID)
		if err != nil {
			slog.Error("Failed to load read files for session", "error", err)
		}

		// Read the transcript here, not in Update: a long session is tens
		// of megabytes and would freeze the event loop while it loads.
		messages, err := m.com.Workspace.ListMessages(context.Background(), sessionID)
		if err != nil {
			return util.ReportError(err)
		}

		return loadSessionMsg{
			session:   &session,
			files:     sessionFiles,
			readFiles: readFiles,
			messages:  messages,
		}
	}
	return tea.Batch(load, m.reportCurrentSession(sessionID))
}

// sessionHookPayload converts a session snapshot into the hook payload
// form. Mirrors the agent-side converter; the two packages do not share
// the type.
func sessionHookPayload(s session.Session) *hooks.SessionPayload {
	return &hooks.SessionPayload{
		ID:               s.ID,
		Title:            s.Title,
		MessageCount:     s.MessageCount,
		PromptTokens:     s.PromptTokens,
		CompletionTokens: s.CompletionTokens,
		Cost:             s.Cost,
		UpdatedAt:        s.UpdatedAt,
	}
}

// sessionSwitched reports whether loading next is a session switch:
// any first session counts as a switch, and loading the session that is
// already displayed does not.
func sessionSwitched(current, next *session.Session) bool {
	return current == nil || current.ID != next.ID
}

// preSessionSwitchHooksConfigured reports whether any PreSessionSwitch
// hook is configured.
func (m *UI) preSessionSwitchHooksConfigured() bool {
	cfg := m.com.Config()
	return cfg != nil && len(cfg.Hooks[hooks.EventPreSessionSwitch]) > 0
}

// preSessionSwitchHooksAsync reports whether every configured
// PreSessionSwitch hook is asynchronous.
func (m *UI) preSessionSwitchHooksAsync() bool {
	cfg := m.com.Config()
	if cfg == nil {
		return false
	}
	return hooks.AllAsync(cfg.Hooks[hooks.EventPreSessionSwitch])
}

// applyLoadedSession applies a loaded session to the UI: the session
// reference, chat list, LSPs, prompt history, and pending bang command.
// The synchronous switch path defers this until its hooks finish.
func (m *UI) applyLoadedSession(msg loadSessionMsg) []tea.Cmd {
	var cmds []tea.Cmd
	if m.forceCompactMode {
		m.isCompact = true
	}
	// Plan mode is scoped to the session it was enabled in: switching
	// to another session falls back to code mode and drops any pending
	// plan handoff. (Loading the session that was just created for the
	// first plan-mode prompt is not a switch; the IDs match then.)
	if m.session == nil || m.session.ID != msg.session.ID {
		if cmd := m.resetPlanModeState(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	m.setState(uiChat, m.focus)
	m.session = msg.session
	m.sidebarOffset = 0
	m.sessionFiles = msg.files
	// Session switch: the memoized busy state and queued prompts
	// belong to the previous session. Drop them and re-fetch
	// off-thread so the queue pill and esc behavior track the new
	// session instead of a stale one.
	m.invalidateBusyCaches()
	m.invalidatePromptQueue()
	m.promptQueue = 0
	m.promptQueueItems = nil
	m.promptQueueCheckedAt = time.Time{}
	if cmd := m.dispatchBusyRefresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.dispatchPromptQueueRefresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, m.startLSPs(msg.lspFilePaths()))
	msgs := msg.messages
	if cmd := m.setSessionMessages(msgs); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.restoreModelFromSession(msgs); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.autoExpandPillsIfReasonable(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// If a bang command was issued before the session finished
	// loading, start it now that the chat list is stable.
	if m.pendingBangCommand != "" {
		cmds = append(cmds, m.runShellCommandInternal(m.pendingBangCommand, true))
		m.pendingBangCommand = ""
	}
	if hasInProgressTodo(m.session.Todos) {
		// only start spinner if there is an in-progress todo
		if m.isAgentBusy() {
			m.todoIsSpinning = true
			cmds = append(cmds, m.todoSpinner.Tick)
		}
		m.updateLayoutAndSize()
	}
	// Reload prompt history for the new session.
	m.historyReset()
	cmds = append(cmds, m.loadPromptHistory())
	m.updateLayoutAndSize()
	return cmds
}

// switchHookLocalID is the chat item id prefix for transient
// PreSessionSwitch hook lines. The lines are not persisted: they live
// in the live chat list only and vanish on reload.
const switchHookLocalID = "switch-hook"

// switchHookGen increments on every switch fire so a completion from an
// abandoned switch never updates the new switch's lines.
var switchHookGen atomic.Int64

// switchHookDoneMsg reports that synchronous PreSessionSwitch hooks
// finished; the deferred session load applies now. runs is the per-hook
// result, displayed as transient lines in the new session.
type switchHookDoneMsg struct {
	gen  int64
	runs []message.HookRun
	load loadSessionMsg
}

// switchHookPendingMsg reports that asynchronous PreSessionSwitch hooks
// started; the pending lines append to the already-displayed session.
type switchHookPendingMsg struct {
	gen  int64
	runs []message.HookRun
}

// switchHookRunsMsg reports that asynchronous PreSessionSwitch hooks
// finished; the pending lines update in place with the outcomes.
type switchHookRunsMsg struct {
	gen  int64
	runs []message.HookRun
}

// fireSessionSwitchHook returns the commands that run the
// PreSessionSwitch lifecycle hook when the displayed session changes:
// one call carries the abandoned session and the newly active one, and
// a first session has no previous side. Synchronous hooks block the
// switch: the command finishes the hooks first, and the caller applies
// the loaded session when the done message arrives. Asynchronous hooks
// switch immediately and surface a pending line that updates in place.
// The command is nil when no hooks are configured.
func (m *UI) fireSessionSwitchHook(previous, next *session.Session, load loadSessionMsg) tea.Cmd {
	cfg := m.com.Config()
	if cfg == nil {
		return nil
	}
	eventHooks := cfg.Hooks[hooks.EventPreSessionSwitch]
	if len(eventHooks) == 0 {
		return nil
	}
	cwd := m.com.Workspace.WorkingDir()
	runner := hooks.NewRunner(eventHooks, cwd, cwd)

	var previousPayload, nextPayload *hooks.SessionPayload
	if previous != nil {
		previousPayload = sessionHookPayload(*previous)
	}
	if next != nil {
		nextPayload = sessionHookPayload(*next)
	}
	sessionID := ""
	if next != nil {
		sessionID = next.ID
	}
	gen := switchHookGen.Add(1)

	if hooks.AllAsync(eventHooks) {
		pending := func() tea.Msg {
			runs := make([]message.HookRun, 0, len(eventHooks))
			seen := make(map[string]bool, len(eventHooks))
			for _, h := range eventHooks {
				if seen[h.Command] {
					continue
				}
				seen[h.Command] = true
				runs = append(runs, message.HookRun{
					Event: hooks.EventPreSessionSwitch, Name: h.DisplayName(),
					Decision: "none", Status: hooks.StatusRunning, Async: true,
				})
			}
			return switchHookPendingMsg{gen: gen, runs: runs}
		}
		done := func() tea.Msg {
			return switchHookRunsMsg{gen: gen, runs: asyncHookRuns(m.runSwitchHook(runner, sessionID, previousPayload, nextPayload))}
		}
		return tea.Batch(pending, done)
	}

	return func() tea.Msg {
		runs := m.runSwitchHook(runner, sessionID, previousPayload, nextPayload)
		return switchHookDoneMsg{gen: gen, runs: runs, load: load}
	}
}

// asyncHookRuns marks every run as fired asynchronously so the
// transcript line carries the status glyph; the glyph only appears on
// fire-and-forget hooks.
func asyncHookRuns(runs []message.HookRun) []message.HookRun {
	for i := range runs {
		runs[i].Async = true
	}
	return runs
}

// runSwitchHook runs the PreSessionSwitch hooks and converts the result
// into transcript runs; a failed runner records no runs.
func (m *UI) runSwitchHook(runner *hooks.Runner, sessionID string, previous, next *hooks.SessionPayload) []message.HookRun {
	result, err := runner.RunLifecycle(context.Background(), hooks.EventPreSessionSwitch, sessionID,
		hooks.SessionEventData{Previous: previous, New: next})
	if err != nil {
		slog.Error("Session switch hook runner failed", "event", hooks.EventPreSessionSwitch, "session_id", sessionID, "error", err)
		return nil
	}
	runs := make([]message.HookRun, len(result.Hooks))
	for i, h := range result.Hooks {
		runs[i] = message.HookRun{Event: hooks.EventPreSessionSwitch, Name: h.Name, Decision: h.Decision, Status: h.Status}
	}
	return runs
}

// appendSwitchHookLines appends the transient switch hook lines to the
// chat list for the currently displayed session.
func (m *UI) appendSwitchHookLines(runs []message.HookRun) {
	if len(runs) == 0 {
		return
	}
	items := make([]chat.MessageItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, chat.NewHookItem(m.com.Styles, switchHookLocalID, message.HookContent{
			Event: hooks.EventPreSessionSwitch,
			Hooks: []message.HookRun{run},
		}))
	}
	m.chat.AppendMessages(items...)
	m.chat.ScrollToBottom()
}

// updateSwitchHookLines refreshes the transient switch hook lines in
// place when an asynchronous run completes. Stale completions from an
// abandoned switch are ignored.
func (m *UI) updateSwitchHookLines(msg switchHookRunsMsg) {
	if msg.gen != switchHookGen.Load() {
		return
	}
	item, ok := m.chat.MessageItem(chat.HookItemID(switchHookLocalID, hooks.EventPreSessionSwitch)).(*chat.HookItem)
	if !ok {
		return
	}
	item.SetContent(message.HookContent{
		Event: hooks.EventPreSessionSwitch,
		Hooks: msg.runs,
	})
}

// reportCurrentSession returns a fire-and-forget tea.Cmd that
// informs the workspace which session this client is currently
// viewing. Errors are logged at debug only; the call is a hint
// for server-side presence tracking, not correctness-critical
// state.
func (m *UI) reportCurrentSession(sessionID string) tea.Cmd {
	return func() tea.Msg {
		if err := m.com.Workspace.SetCurrentSession(context.Background(), sessionID); err != nil {
			slog.Debug("Failed to report current session", "session_id", sessionID, "error", err)
		}
		return nil
	}
}

func (m *UI) loadSessionFiles(sessionID string) ([]SessionFile, error) {
	files, err := m.com.Workspace.ListSessionHistory(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}

	filesByPath := make(map[string][]history.File)
	for _, f := range files {
		filesByPath[f.Path] = append(filesByPath[f.Path], f)
	}
	sessionFiles := make([]SessionFile, 0, len(filesByPath))
	for _, versions := range filesByPath {
		if len(versions) == 0 {
			continue
		}

		first := versions[0]
		last := versions[0]
		for _, v := range versions {
			if v.Version < first.Version {
				first = v
			}
			if v.Version > last.Version {
				last = v
			}
		}

		_, additions, deletions := diff.GenerateDiff(first.Content, last.Content, first.Path)

		sessionFiles = append(sessionFiles, SessionFile{
			FirstVersion:  first,
			LatestVersion: last,
			Additions:     additions,
			Deletions:     deletions,
		})
	}

	slices.SortFunc(sessionFiles, func(a, b SessionFile) int {
		if a.LatestVersion.UpdatedAt > b.LatestVersion.UpdatedAt {
			return -1
		}
		if a.LatestVersion.UpdatedAt < b.LatestVersion.UpdatedAt {
			return 1
		}
		return 0
	})
	return sessionFiles, nil
}

// handleFileEvent processes file change events and updates the session file
// list with new or updated file information.
func (m *UI) handleFileEvent(file history.File) tea.Cmd {
	if m.session == nil || file.SessionID != m.session.ID {
		return nil
	}

	return func() tea.Msg {
		sessionFiles, err := m.loadSessionFiles(m.session.ID)
		// could not load session files
		if err != nil {
			return util.NewErrorMsg(err)
		}

		return sessionFilesUpdatesMsg{
			sessionFiles: sessionFiles,
		}
	}
}

// filesInfo renders the modified files section for the sidebar, showing files
// with their addition/deletion counts.
func (m *UI) filesInfo(cwd string, width, maxItems int, isSection bool) string {
	t := m.com.Styles

	title := t.Files.SectionTitle.Render("Modified Files")
	if isSection {
		title = common.Section(t, "Modified Files", width)
	}
	list := t.Files.EmptyMessage.Render("None")
	var filesWithChanges []SessionFile
	for _, f := range m.sessionFiles {
		if f.Additions == 0 && f.Deletions == 0 {
			continue
		}
		filesWithChanges = append(filesWithChanges, f)
	}
	if len(filesWithChanges) > 0 {
		list = fileList(t, cwd, filesWithChanges, width, maxItems)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

// fileList renders a list of files with their diff statistics, truncating to
// maxItems and showing a "...and N more" message if needed.
func fileList(t *styles.Styles, cwd string, filesWithChanges []SessionFile, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}
	var renderedFiles []string
	filesShown := 0

	for _, f := range filesWithChanges {
		// Skip files with no changes
		if filesShown >= maxItems {
			break
		}

		// Build stats string with colors
		var statusParts []string
		if f.Additions > 0 {
			statusParts = append(statusParts, t.Files.Additions.Render(fmt.Sprintf("+%d", f.Additions)))
		}
		if f.Deletions > 0 {
			statusParts = append(statusParts, t.Files.Deletions.Render(fmt.Sprintf("-%d", f.Deletions)))
		}
		extraContent := strings.Join(statusParts, " ")

		// Format file path
		filePath := f.FirstVersion.Path
		if rel, err := filepath.Rel(cwd, filePath); err == nil {
			filePath = rel
		}
		filePath = fsext.DirTrim(filePath, 2)
		suffix := ""
		if extraContent != "" {
			suffix = " " + extraContent
		}
		maxPathWidth := max(width-lipgloss.Width(suffix), 0)
		filePath = ansi.Truncate(filePath, maxPathWidth, "…")

		line := t.Files.Path.Render(filePath)
		if extraContent != "" {
			line = fmt.Sprintf("%s %s", line, extraContent)
		}

		renderedFiles = append(renderedFiles, line)
		filesShown++
	}

	if len(filesWithChanges) > maxItems {
		remaining := len(filesWithChanges) - maxItems
		renderedFiles = append(renderedFiles, t.Files.TruncationHint.Render(fmt.Sprintf("…and %d more", remaining)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, renderedFiles...)
}

// startLSPs starts LSP servers for the given file paths.
func (m *UI) startLSPs(paths []string) tea.Cmd {
	if len(paths) == 0 {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		for _, path := range paths {
			m.com.Workspace.LSPStart(ctx, path)
		}
		return nil
	}
}
