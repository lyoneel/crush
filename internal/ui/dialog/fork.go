package dialog

import (
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fork"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// ActionRunForkCommand runs the fork command registered under ID.
type ActionRunForkCommand struct {
	ID string
}

// forkCommandItems returns a palette item for every command registered
// in the fork registry, labeled from the live config.
func forkCommandItems(t *styles.Styles, cfg *config.Config) []*CommandItem {
	registered := fork.Commands()
	items := make([]*CommandItem, 0, len(registered))
	for _, c := range registered {
		items = append(items, NewCommandItem(t, "fork_"+c.ID, c.Title(cfg), "", ActionRunForkCommand{ID: c.ID}))
	}
	return items
}
