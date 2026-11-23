package model

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/fork"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// forkCommandDoneMsg reports that a registered fork command finished.
type forkCommandDoneMsg struct {
	info string
	err  error
}

// runForkCommand closes the palette and runs the registered fork command
// off the UI goroutine.
func (m *UI) runForkCommand(action dialog.ActionRunForkCommand) tea.Cmd {
	m.dialog.CloseDialog(dialog.CommandsID)
	c, ok := fork.LookupCommand(action.ID)
	if !ok {
		return nil
	}
	ws := m.com.Workspace
	return func() tea.Msg {
		info, err := c.Run(ws)
		return forkCommandDoneMsg{info: info, err: err}
	}
}

// handleForkCommandDone re-syncs the fork state with the reloaded config,
// re-lays out the screen so flag-driven sections redraw, and reports the
// outcome. A credits balance that just became visible is fetched again,
// since fetches were skipped while it was hidden.
func (m *UI) handleForkCommandDone(msg forkCommandDoneMsg) tea.Cmd {
	fork.Sync(m.com.Config())
	m.updateLayoutAndSize()
	if msg.err != nil {
		return util.ReportError(msg.err)
	}
	var cmds []tea.Cmd
	if msg.info != "" {
		cmds = append(cmds, util.ReportInfo(msg.info))
	}
	if m.com.ShowCredits() {
		m.updateHyperCredits()
		cmds = append(cmds, m.fetchHyperCredits())
	}
	return tea.Batch(cmds...)
}

// visibleCredits returns the credits balance to render, or nil when the
// selected model consumes no credits or the display is gated off.
func (m *UI) visibleCredits() *int {
	if !m.com.ShowCredits() {
		return nil
	}
	return m.hyperCredits
}
