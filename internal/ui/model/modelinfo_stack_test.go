package model

import (
	"strings"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type twoModelWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w twoModelWorkspace) Config() *config.Config {
	return w.cfg
}

func twoModelConfig() *config.Config {
	provider := config.ProviderConfig{
		ID:   "test",
		Name: "Test Provider",
		Models: []catwalk.Model{
			{ID: "big-model", Name: "Big Model", CanReason: true, ReasoningLevels: []string{"low", "high"}, DefaultReasoningEffort: "high", ContextWindow: 1000},
			{ID: "small-model", Name: "Small Model", CanReason: true, ReasoningLevels: []string{"low", "high"}, DefaultReasoningEffort: "low", ContextWindow: 500},
		},
	}
	cfg := &config.Config{}
	cfg.Providers = csync.NewMap[string, config.ProviderConfig]()
	cfg.Providers.Set("test", provider)
	cfg.Models = map[config.SelectedModelType]config.SelectedModel{
		config.SelectedModelTypeLarge: {Provider: "test", Model: "big-model"},
		config.SelectedModelTypeSmall: {Provider: "test", Model: "small-model"},
	}
	return cfg
}

func stackTestModel(t *testing.T) *UI {
	t.Helper()
	m := newTestUI()
	m.com.Workspace = twoModelWorkspace{cfg: twoModelConfig()}
	m.agentReady = true
	m.agentModel = workspace.AgentModel{
		CatwalkCfg: catwalk.Model{ID: "big-model", Name: "Big Model", CanReason: true, ReasoningLevels: []string{"low", "high"}, DefaultReasoningEffort: "high", ContextWindow: 1000},
		ModelCfg:   config.SelectedModel{Provider: "test", Model: "big-model"},
	}
	return m
}

func TestModelInfoStacksSmallModelBelowMain(t *testing.T) {
	m := stackTestModel(t)
	out := m.modelInfo(40, true)

	big := strings.Index(out, "Big Model")
	small := strings.Index(out, "Small Model")
	require.GreaterOrEqual(t, big, 0, "main model must render")
	require.GreaterOrEqual(t, small, 0, "small model must render")
	require.Less(t, big, small, "small model must render below the main model")
	require.Contains(t, out, "{High}", "main model effort must render inline in braces")
	require.Contains(t, out, "{Low}", "small model effort must render inline in braces")
	require.Contains(t, out, "[Test Provider]", "the provider must render in brackets")
	require.NotContains(t, out, "via", "the via word must be gone")
	require.Contains(t, out, styles.ModelIcon, "the main model must use the main icon")
	require.Equal(t, 2, strings.Count(out, styles.ModelIcon), "both model lines must use the same icon glyph")

	mainStyled := m.com.Styles.ModelInfo.Icon.Render(styles.ModelIcon)
	smallStyled := m.com.Styles.ModelInfo.SmallIcon.Render(styles.ModelIcon)
	require.Equal(t, 1, strings.Count(out, mainStyled), "the main model icon must carry the main icon color")
	require.Equal(t, 1, strings.Count(out, smallStyled), "the small model icon must carry the small icon color")
}

func TestModelInfoRendersIdenticalSmallModel(t *testing.T) {
	m := stackTestModel(t)
	m.com.Config().Models[config.SelectedModelTypeSmall] = config.SelectedModel{Provider: "test", Model: "big-model"}
	out := m.modelInfo(40, true)
	require.Equal(t, 2, strings.Count(out, "Big Model"), "the small model line renders even when it matches the main model")
}

func TestSelectedSmallModelReadsConfig(t *testing.T) {
	m := stackTestModel(t)
	small := m.selectedSmallModel()
	require.NotNil(t, small)
	require.Equal(t, "small-model", small.ModelCfg.Model)
	require.Equal(t, "Small Model", small.CatwalkCfg.Name)
}

func TestModelInfoPlacesBalanceBeforeBothModels(t *testing.T) {
	m := stackTestModel(t)
	m.hyperCredits = intPtr(42)
	m.com.Config().Providers.Set("hyper", config.ProviderConfig{ID: "hyper", Name: "Charm Hyper"})
	m.com.Config().Models[config.SelectedModelTypeLarge] = config.SelectedModel{Provider: "hyper", Model: "Hyper/big-model"}
	m.agentModel = workspace.AgentModel{
		CatwalkCfg: catwalk.Model{ID: "Hyper/big-model", Name: "Big Model"},
		ModelCfg:   config.SelectedModel{Provider: "hyper", Model: "Hyper/big-model"},
	}

	sidebar := m.modelInfo(40, true)
	balance := strings.Index(sidebar, "Hypercredits")
	require.GreaterOrEqual(t, balance, 0, "the balance must render")
	require.Less(t, balance, strings.Index(sidebar, "Big Model"),
		"the sidebar layout must render the balance above the models, below the folder line")

	plain := m.modelInfo(40, false)
	balance = strings.Index(plain, "Hypercredits")
	require.GreaterOrEqual(t, balance, 0, "the balance must render")
	require.Less(t, balance, strings.Index(plain, "Big Model"),
		"the landing layout must also render the balance before the models")
}

func intPtr(i int) *int { return &i }

func TestModelInfoSidebarLayoutMovesContextBelowSmall(t *testing.T) {
	m := stackTestModel(t)
	m.session = &session.Session{ID: "s1", PromptTokens: 100, CompletionTokens: 10}

	sidebar := m.modelInfo(40, true)
	require.GreaterOrEqual(t, strings.Index(sidebar, "(110)"), 0, "the context summary must render")
	require.Less(t, strings.Index(sidebar, "Small Model"), strings.Index(sidebar, "(110)"),
		"the sidebar layout must render the context below the small model")

	plain := m.modelInfo(40, false)
	require.GreaterOrEqual(t, strings.Index(plain, "(110)"), 0, "the context summary must render")
	require.Less(t, strings.Index(plain, "(110)"), strings.Index(plain, "Small Model"),
		"the landing layout must keep the context attached to the main model")
}

func TestModelInfoShowsEnforcedLowestEffortForSmallModel(t *testing.T) {
	m := stackTestModel(t)
	cfg := m.com.Config()
	cfg.Models[config.SelectedModelTypeSmall] = config.SelectedModel{
		Provider:        "test",
		Model:           "small-model",
		ReasoningEffort: "high",
	}

	out := m.modelInfo(40, true)
	require.Contains(t, out, "{Low}", "the small model must show the enforced lowest effort")
	require.Equal(t, 1, strings.Count(out, "{High}"), "only the main model may show its own effort")
}
