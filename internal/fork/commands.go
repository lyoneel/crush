package fork

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
)

// Env is what a registered command works with: the live config and a way
// to persist settings. The UI workspace satisfies it.
type Env interface {
	Config() *config.Config
	SetConfigField(scope config.Scope, key string, value any) error
}

// Command is a palette entry that a fork branch registers from its own
// file, usually in an init function.
type Command struct {
	// ID is unique across registered commands. The palette item id is
	// "fork_" + ID.
	ID string
	// Title returns the palette label. It gets the live config (which may
	// be nil) so a label can follow the current state.
	Title func(cfg *config.Config) string
	// Run performs the command off the UI goroutine. The returned text,
	// when not empty, is shown as an info message. The UI re-syncs fork
	// state and re-renders after Run returns.
	Run func(env Env) (string, error)
}

var (
	commandsMu sync.RWMutex
	commands   = map[string]Command{}
)

// RegisterCommand adds c to the palette registry. It panics on an empty
// id, a missing Title or Run, or a duplicate id: all are programming
// errors caught at startup.
func RegisterCommand(c Command) {
	if strings.TrimSpace(c.ID) == "" || c.Title == nil || c.Run == nil {
		panic(fmt.Sprintf("fork: invalid command registration %q", c.ID))
	}
	commandsMu.Lock()
	defer commandsMu.Unlock()
	if _, dup := commands[c.ID]; dup {
		panic(fmt.Sprintf("fork: command %q registered twice", c.ID))
	}
	commands[c.ID] = c
}

// Commands returns the registered commands sorted by id, so the palette
// order does not depend on file init order.
func Commands() []Command {
	commandsMu.RLock()
	defer commandsMu.RUnlock()
	out := make([]Command, 0, len(commands))
	for _, c := range commands {
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b Command) int { return strings.Compare(a.ID, b.ID) })
	return out
}

// LookupCommand returns the command registered under id.
func LookupCommand(id string) (Command, bool) {
	commandsMu.RLock()
	defer commandsMu.RUnlock()
	c, ok := commands[id]
	return c, ok
}
