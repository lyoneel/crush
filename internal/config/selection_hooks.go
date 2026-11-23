package config

import "sync"

// SelectionHook adjusts the model selection a slot actually runs with,
// without touching the persisted selection. Fork features register one
// from their own file; the coordinator applies the hooks when it builds a
// slot's model and the UI applies them when it displays one, so both
// always agree on what gets sent.
type SelectionHook func(cfg *Config, slot SelectedModelType, sel SelectedModel) SelectedModel

var (
	selectionHooksMu sync.RWMutex
	selectionHooks   []SelectionHook
)

// RegisterSelectionHook adds h to the selection hooks, applied in
// registration order. A nil hook is ignored.
func RegisterSelectionHook(h SelectionHook) {
	if h == nil {
		return
	}
	selectionHooksMu.Lock()
	defer selectionHooksMu.Unlock()
	selectionHooks = append(selectionHooks, h)
}

// EffectiveSelection returns sel as the slot will run it, after every
// registered selection hook. With no hooks it returns sel unchanged.
func (c *Config) EffectiveSelection(slot SelectedModelType, sel SelectedModel) SelectedModel {
	selectionHooksMu.RLock()
	defer selectionHooksMu.RUnlock()
	for _, h := range selectionHooks {
		sel = h(c, slot, sel)
	}
	return sel
}
