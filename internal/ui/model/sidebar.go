package model

import (
	"cmp"
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	mcp "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/logo"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/layout"
)

// selectedSmallModel returns the currently selected small model from the
// live config. Unlike the large model it needs no agent probe: the
// config carries the selection, and the header already reads it on every
// frame.
func (m *UI) selectedSmallModel() *workspace.AgentModel {
	cfg := m.com.Config()
	if cfg == nil {
		return nil
	}
	sel, ok := cfg.Models[config.SelectedModelTypeSmall]
	if !ok {
		return nil
	}
	catwalkModel := cfg.GetModel(sel.Provider, sel.Model)
	if catwalkModel == nil {
		return nil
	}
	// Mirror the coordinator's enforcement: the small model always runs
	// at its lowest reasoning effort, so the display shows what actually
	// gets sent.
	if effort := config.LowestReasoningEffort(catwalkModel); effort != "" {
		sel.ReasoningEffort = effort
	}
	return &workspace.AgentModel{CatwalkCfg: *catwalkModel, ModelCfg: sel}
}

// modelInfoLine renders one model information block in the shared
// format. usage controls the context and cost summary, which only
// applies to the model the session usage is tracked against.
func (m *UI) modelInfoLine(width int, model *workspace.AgentModel, usage bool, icon string, smallModel bool) string {
	effortInfo := ""
	providerName := ""

	// Get provider name first
	if providerConfig, ok := m.com.Config().Providers.Get(model.ModelCfg.Provider); ok {
		providerName = providerConfig.Name

		// Only show the effort inline if the model can reason
		if model.CatwalkCfg.CanReason {
			if len(model.CatwalkCfg.ReasoningLevels) == 0 {
				if model.ModelCfg.Think {
					effortInfo = "Thinking On"
				} else {
					effortInfo = "Thinking Off"
				}
			} else {
				effortInfo = common.FormatReasoningEffort(cmp.Or(model.ModelCfg.ReasoningEffort, model.CatwalkCfg.DefaultReasoningEffort))
			}
		}
	}

	var modelContext *common.ModelContextInfo
	if usage && m.session != nil {
		modelContext = &common.ModelContextInfo{
			ContextUsed:    m.session.CompletionTokens + m.session.PromptTokens,
			Cost:           m.session.Cost,
			ModelContext:   model.CatwalkCfg.ContextWindow,
			EstimatedUsage: m.session.EstimatedUsage,
		}
	}

	return common.ModelInfo(m.com.Styles, model.CatwalkCfg.Name, providerName, effortInfo, modelContext, width, nil, false, icon, smallModel)
}

// modelInfo renders the current model information including the inline
// effort and context usage/cost, with the small model in the same
// format on the line below, and the hypercredit balance before both,
// matching the sidebar placement. With the sidebar layout the context
// and cost summary moves below the small model; the landing view keeps
// the context attached to the main model line.
func (m *UI) modelInfo(width int, sidebarLayout bool) string {
	large := m.selectedLargeModel()
	small := m.selectedSmallModel()

	var credits *int
	if large != nil && m.com.IsHyperCredits() && m.showHyperCredits {
		credits = m.hyperCredits
	}

	var blocks []string
	if credits != nil {
		blocks = append(blocks, common.HypercreditBalance(m.com.Styles, *credits), "")
	}
	if large != nil {
		blocks = append(blocks, m.modelInfoLine(width, large, !sidebarLayout, styles.ModelIcon, false))
	}
	if small != nil {
		blocks = append(blocks, m.modelInfoLine(width, small, false, styles.ModelIcon, true))
	}
	if sidebarLayout && large != nil && m.session != nil {
		blocks = append(blocks, common.ModelContextSummary(m.com.Styles, common.ModelContextInfo{
			ContextUsed:    m.session.CompletionTokens + m.session.PromptTokens,
			Cost:           m.session.Cost,
			ModelContext:   large.CatwalkCfg.ContextWindow,
			EstimatedUsage: m.session.EstimatedUsage,
		}))
	}
	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

// updateSidebarScrollState renders the sidebar content and computes scroll
// state (scrollability, max offset, clamp) before drawing. This keeps all
// state mutation in the update path rather than in the draw function.
func (m *UI) updateSidebarScrollState() {
	if m.session == nil || m.isCompact {
		return
	}

	const logoHeightBreakpoint = 30

	t := m.com.Styles
	width := m.layout.sidebar.Dx()
	height := m.layout.sidebar.Dy()

	contentWidth := max(width-2, 1)

	title := t.Sidebar.SessionTitle.Width(contentWidth).MaxHeight(2).Render(m.session.Title)
	cwd := common.PrettyPathWithBranch(t, m.com.Workspace.WorkingDir(), m.gitBranch, contentWidth)
	sidebarLogo := m.sidebarLogo
	if height < logoHeightBreakpoint {
		sidebarLogo = lipgloss.JoinVertical(lipgloss.Left, logo.SmallRender(m.com.Styles, contentWidth, logo.Opts{
			Hyper: m.com.IsHyper(),
		}), "")
	}

	var logoRect, contentRect image.Rectangle
	layout.Vertical(
		layout.Len(lipgloss.Height(sidebarLogo)),
		layout.Fill(1),
	).Split(m.layout.sidebar).Assign(&logoRect, &contentRect)

	contentHeight := contentRect.Dy()

	// Render all items without truncation; virtual scrolling handles overflow.
	// The minimalism toggle switches the filter between the F-8 hidden
	// states and the full pre-F-8 list.
	lspSection := m.lspInfo(contentWidth, len(m.lspStates), true, m.sidebarMinimalism)
	mcpSection := m.mcpInfo(contentWidth, mcpCount(m.com.Config().MCP.Sorted(), m.mcpStates), true, m.sidebarMinimalism)
	skillsSection := m.skillsInfo(contentWidth, len(m.skillStatusItems()), true)
	filesSection := m.filesInfo(m.com.Workspace.WorkingDir(), contentWidth, fileChangeCount(m.sessionFiles), true)

	// Build the scrollable content.
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		cwd,
		"",
		m.modelInfo(contentWidth, true),
		"",
		filesSection,
		"",
		lspSection,
		"",
		mcpSection,
		"",
		skillsSection,
	)

	totalLines := strings.Count(content, "\n") + 1
	m.sidebarContent = content
	m.sidebarTotalLines = totalLines
	m.sidebarContentWidth = contentWidth
	m.sidebarContentHeight = contentHeight
	m.sidebarDrawLogo = sidebarLogo
	m.sidebarScrollable = totalLines > contentHeight
	m.sidebarMaxOffsetVal = max(0, totalLines-contentHeight)

	// If the sidebar is focused but no longer scrollable (e.g. after a
	// resize), return focus to the chat.
	if m.focus == uiFocusSidebar && !m.sidebarScrollable {
		m.focus = uiFocusMain
		m.chat.Focus()
	}

	// Clamp sidebarOffset.
	if m.sidebarOffset > m.sidebarMaxOffsetVal {
		m.sidebarOffset = m.sidebarMaxOffsetVal
	}
}

// drawSidebar renders the chat sidebar with a fixed logo and a
// virtual-scrolling content area with an auto-hiding scrollbar. While the
// sidebar is focused, the scrollbar stays visible.
func (m *UI) drawSidebar(scr uv.Screen, area uv.Rectangle) {
	if m.session == nil {
		return
	}

	sidebarLogo := m.sidebarDrawLogo
	contentWidth := m.sidebarContentWidth
	contentHeight := m.sidebarContentHeight
	totalLines := m.sidebarTotalLines

	var logoRect, contentRect image.Rectangle
	layout.Vertical(
		layout.Len(lipgloss.Height(sidebarLogo)),
		layout.Fill(1),
	).Split(area).Assign(&logoRect, &contentRect)

	// Slice visible lines.
	end := min(m.sidebarOffset+contentHeight, totalLines)
	lines := strings.Split(m.sidebarContent, "\n")
	visibleLines := lines[m.sidebarOffset:end]
	visibleStr := strings.Join(visibleLines, "\n")

	// Determine scrollbar visibility: always visible when focused, otherwise
	// auto-hide.
	scrollbarVisible := totalLines > contentHeight && (m.sidebarScrollbarVisible || m.focus == uiFocusSidebar)

	// Draw the fixed logo.
	uv.NewStyledString(
		lipgloss.NewStyle().
			MaxWidth(contentWidth).
			MaxHeight(lipgloss.Height(sidebarLogo)).
			Render(sidebarLogo),
	).Draw(scr, logoRect)

	// Draw the visible content in the scrollable area.
	uv.NewStyledString(
		lipgloss.NewStyle().
			MaxWidth(contentWidth).
			MaxHeight(contentHeight).
			Render(visibleStr),
	).Draw(scr, contentRect)

	// Draw scrollbar in the reserved column.
	if scrollbarVisible {
		scrollbar := common.Scrollbar(m.com.Styles, contentHeight, totalLines, contentHeight, m.sidebarOffset)
		if scrollbar != "" {
			scrollbarArea := image.Rectangle{
				Min: image.Point{X: area.Max.X - 1, Y: contentRect.Min.Y},
				Max: image.Point{X: area.Max.X, Y: area.Max.Y},
			}
			uv.NewStyledString(scrollbar).Draw(scr, scrollbarArea)
		}
	}
}

// fileChangeCount returns the number of session files with non-zero additions
// or deletions.
func fileChangeCount(files []SessionFile) int {
	count := 0
	for _, f := range files {
		if f.Additions == 0 && f.Deletions == 0 {
			continue
		}
		count++
	}
	return count
}

// mcpCount returns the number of MCP servers that have a state entry.
func mcpCount(mcpCfgs []config.MCP, states map[string]mcp.ClientInfo) int {
	count := 0
	for _, cfg := range mcpCfgs {
		if _, ok := states[cfg.Name]; ok {
			count++
		}
	}
	return count
}
