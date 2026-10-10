package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/key"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type modelsTestWorkspace struct {
	workspace.Workspace
}

func (w *modelsTestWorkspace) Config() *config.Config {
	cfg := &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	cfg.Models = make(map[config.SelectedModelType]config.SelectedModel)
	return cfg
}

func newTestModels(modelType ModelType) *Models {
	s := styles.CharmtonePantera()

	m := &Models{}
	m.com = &common.Common{
		Workspace: &modelsTestWorkspace{},
		Styles:    &s,
	}
	m.isOnboarding = false
	m.modelType = modelType
	m.list = NewModelsList(&s)
	m.list.Focus()

	m.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab", "shift+tab"),
		key.WithHelp("⇥/⇤", "toggle type"),
	)
	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("⏎", "select"),
	)
	m.keyMap.SelectBoth = key.NewBinding(
		key.WithKeys("shift+enter"),
		key.WithHelp("shift+enter", "select both"),
	)
	m.keyMap.Edit = key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("Ctl+e", "edit"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	m.keyMap.Close = CloseKey

	prov := catwalk.Provider{ID: catwalk.InferenceProvider("test-provider"), Name: "Test Provider"}
	item := NewModelItem(&s, prov, catwalk.Model{ID: "test-model", Name: "Test Model"}, modelType, false)
	group := NewModelGroup(&s, "Test Provider", true, item)
	m.list.SetGroups(group)
	m.list.SetSelected(0)

	return m
}

func TestModelsShiftEnterAppliesBoth(t *testing.T) {
	t.Parallel()

	for _, modelType := range []ModelType{ModelTypeLarge, ModelTypeSmall} {
		m := newTestModels(modelType)

		action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
		selected, ok := action.(ActionSelectModel)
		require.True(t, ok)
		require.True(t, selected.ApplyBoth)
		require.False(t, selected.ReAuthenticate)
		require.Equal(t, "test-model", selected.Model.Model)
		require.Equal(t, "test-provider", selected.Model.Provider)
		want := config.SelectedModelTypeLarge
		if modelType == ModelTypeSmall {
			want = config.SelectedModelTypeSmall
		}
		require.Equal(t, want, selected.ModelType)
	}
}

func TestModelsEnterDoesNotApplyBoth(t *testing.T) {
	t.Parallel()

	m := newTestModels(ModelTypeLarge)

	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := action.(ActionSelectModel)
	require.True(t, ok)
	require.False(t, selected.ApplyBoth)
	require.False(t, selected.ReAuthenticate)
}

func TestModelsCtrlJDoesNotApplyBoth(t *testing.T) {
	t.Parallel()

	m := newTestModels(ModelTypeLarge)

	action := m.HandleMsg(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	_, ok := action.(ActionSelectModel)
	require.False(t, ok)
}

func TestModelsCtrlYDoesNotApplyBoth(t *testing.T) {
	t.Parallel()

	m := newTestModels(ModelTypeLarge)

	action := m.HandleMsg(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	selected, ok := action.(ActionSelectModel)
	require.True(t, ok)
	require.False(t, selected.ApplyBoth)
}

func TestModelsShortHelpIncludesBothBinding(t *testing.T) {
	t.Parallel()

	m := newTestModels(ModelTypeLarge)

	help := m.ShortHelp()
	require.Contains(t, help, m.keyMap.SelectBoth)
}
