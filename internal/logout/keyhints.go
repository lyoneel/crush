package logout

import (
	"charm.land/bubbles/v2/help"

	"github.com/charmbracelet/crush/internal/ui/common"
)

// conditionalHelpView renders the help view through the symbol
// translation when the package default is on. The standalone TUIs hold
// no config, so the package default decides; the main UI passes the
// config flag instead.
func conditionalHelpView(h help.Model, km help.KeyMap) string {
	if common.SymbolHintsEnabled {
		return h.View(common.NewSymbolKeyMap(km))
	}
	return h.View(km)
}
