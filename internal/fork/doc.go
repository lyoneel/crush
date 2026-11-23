// Package fork holds the shared plumbing that the fork's feature and
// toggle branches build on, so each of them can live in its own files
// without editing the same upstream lines:
//
//   - a command registry for palette entries (RegisterCommand)
//   - helpers over the options.tui.fork flag store (SetFlag, ToggleFlag)
//   - the credits display gate (RegisterCreditsDisplayGate)
//   - the sync step that mirrors flags into process state (Sync,
//     RegisterSyncHook)
//
// See FORK.md at the repository root for the branch model.
package fork
