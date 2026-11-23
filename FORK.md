# How this fork is built

This document explains how the fork of
[charmbracelet/crush](https://github.com/charmbracelet/crush) is laid
out, the rules every fork change follows, and the exact steps to bring
the fork up to date when upstream moves. The [README](README.md) lists
what each feature does; this file is about how the pieces fit
together.

## 1. Branch hierarchy

```text
upstream/main  (charmbracelet/crush)
      │  fast-forward only
      ▼
upstream-main ──────────────────────────────────────────────┐
      │                                                     │
      ├── base/ly-fork-plumbing          layer 1: shared infra
      │     │                                               │
      │     ├── toggle/ly-sidebar-minimalism   layer 2: toggles
      │     │     └── feat/ly-minimalist-sidebar            │
      │     ├── toggle/ly-credits-display                   │
      │     │                                               │
      │     ├── feat/ly-hypercredit-gateway-gate   layer 3: features
      │     ├── feat/ly-small-model-display                 │
      │     ├── feat/ly-small-model-effort                  │
      │     ├── feat/ly-apply-both                          │
      │     ├── feat/ly-symbol-key-hints                    │
      │     ├── feat/ly-code-mode-banner                    │
      │     ├── feat/ly-gateway-discovery-metadata          │
      │     ├── feat/ly-list-nogap                          │
      │     └── feat/ly-mcp-capability-counts               │
      │                                                     │
      └── docs/ly-fork-docs              layer 4: docs      │
                                                            ▼
main  = upstream-main + one squashed commit per branch above
```

Naming:

| Prefix | Layer | Holds |
|--------|-------|-------|
| `upstream-main` | 0 | A 1:1 copy of upstream `main`. Never carries fork commits. |
| `base/ly-*` | 1 | Infrastructure every fork branch may use: the flag store, the command registry, the credits registry, hooks, the version stamp, test fixes. No user-visible feature. |
| `toggle/ly-*` | 2 | A persisted switch plus its palette command. A toggle may gate several features. |
| `feat/ly-*` | 3 | One user-visible feature each. |
| `docs/ly-*` | 4 | The README fork section and this file. Built straight on `upstream-main`, because docs need no code. |
| `main` | - | The default branch: upstream plus everything above. |

Branches are never deleted, and the granular commits on each branch
are kept. `main` is never force-pushed.

## 2. Design principles

### Independent, removable features

Every feature branch must build and pass its tests when it is the only
fork branch merged on top of `base/ly-fork-plumbing` (or on top of its
toggle, for features that a toggle gates). A feature never imports
another feature. When two features need to agree on something (for
example, the small model display showing the effort that the small
model effort feature enforces), the agreement goes through a hook in
the base layer. That way either feature can be dropped without
touching the other.

Keep the upstream surface small. Put new code in new files
(`fork.go`, `*_minimalism.go`, `keyhints.go`, ...) and change upstream
files only at the call sites. Small edits to upstream files are what
make rebases cheap.

### Toggles are prerequisite infrastructure

The dependency runs one way: **toggle → features**. A toggle owns a
flag and a command, and exposes a helper that features ask, such as
`fork.IsSidebarMinimalism(cfg)`. A feature reads the flag through that
helper. It never registers commands for the toggle and never assumes
any other feature exists. The toggle does not know which of its
features are merged, so it still works when none of them are.

A single-purpose feature whose switch only matters to itself (symbol
key hints) keeps its own flag and command inside its feature branch.

### The fork flag store: `options.tui.fork`

All fork switches live in one JSON object instead of new top-level
config fields, so upstream's config struct and schema change only
once:

```json
{
  "options": {
    "tui": {
      "fork": {
        "sidebar_minimalism": true,
        "show_credits": false,
        "symbol_hints": true
      }
    }
  }
}
```

- Code reads a flag with `cfg.ForkFlag(name, default)`. A missing key
  returns the default, so an empty config behaves like a fresh
  install.
- Code writes a flag with `fork.SetFlag` / `fork.ToggleFlag`. These go
  through the workspace's `SetConfigField(scope, "options.tui.fork.<name>", value)`,
  so the value persists and the config reloads in both local and
  client/server mode.
- **Legacy fallback.** Earlier fork builds stored flat keys. While
  the new key is unset, `TUIOptions.UnmarshalJSON` folds the old
  value into the fork map:

  | Legacy key | Fork flag |
  |------------|-----------|
  | `options.tui.sidebar_minimalism` | `sidebar_minimalism` |
  | `options.tui.show_hypercredits` | `show_credits` |
  | `options.tui.symbol_hints` | `symbol_hints` |

  The fold happens at decode time, so it also survives config
  snapshots sent from the server to the client. The mapping lives in
  `internal/config/fork_flags.go` (`legacyForkFlags`).
- `schema.json` gains only the `fork` object (booleans keyed by name).
  Regenerate it with `go run . schema > schema.json`.

### The command registry

`internal/fork/commands.go` holds the palette commands every fork
branch registers from an `init()`:

```go
fork.RegisterCommand(fork.Command{
	ID:    "toggle_sidebar_minimalism",
	Title: func(*config.Config) string { return "Toggle Compact Sidebar" },
	Run: func(env fork.Env) (string, error) {
		on, err := fork.ToggleFlag(env, SidebarMinimalismFlag, true)
		// ... return the info message shown in the status bar
	},
})
```

The base layer wires the registry into the UI once:

- `dialog/fork.go` appends every registered command to the palette.
- `model/fork.go` runs the command, then calls `fork.Sync`, refreshes
  the layout and shows the returned message.

A branch adds a command without editing `ui.go`, `commands.go` or
`actions.go`. Registering an empty or duplicate ID panics at
startup, so collisions show up right away.

`fork.Sync(cfg)` runs at UI start and after every fork command. It
mirrors flags into process state that has no config access, such as
the credits fetch gate and the symbol hint render switch. Branches
add their own steps with `fork.RegisterSyncHook`.

### The multi-router credits registry

Upstream shows the Hyper credit balance only for the direct `hyper`
provider. The base layer turns that check into a list:

- `hyper.RegisterCreditsMatcher(func(provider, model string) bool)`
  adds a router whose models count as credits models. The default
  matcher is the direct Hyper provider. `feat/ly-hypercredit-gateway-gate`
  registers the gateway matcher (`Hyper/` ids or `/hyper/` segments).
- `hyper.IsCreditsModel` asks every matcher. The agent coordinator
  uses it to wrap the model with credit tracking, and the UI uses it
  (`Common.IsCreditsModel`) to decide where the balance can show.
- `fork.RegisterCreditsDisplayGate` adds a display gate.
  `fork.CreditsDisplayEnabled` requires every gate to allow, and
  `Common.ShowCredits()` combines that with `IsCreditsModel`.
  `toggle/ly-credits-display` registers the `show_credits` gate.
- `hyper.SetCreditsFetchEnabled` is the fetch switch that
  `fork.Sync` drives from the display gates. A hidden balance costs no
  fetches.

To add a router, write a matcher in its own file and register it from
`init()`. No UI change is needed.

### Model selection hooks

`config.RegisterSelectionHook` lets a branch change the model a slot
runs with, for example forcing the lowest reasoning effort on the
small model. The coordinator builds models through
`cfg.EffectiveSelection(slot, sel)`, and the sidebar displays the same
effective selection. The display therefore always matches what runs,
without either branch knowing about the other.

## 3. Inventory

| Branch | Base | Purpose | Config key |
|--------|------|---------|------------|
| `upstream-main` | `upstream/main` | 1:1 upstream copy | - |
| `base/ly-fork-plumbing` | `upstream-main` | Fork flag store, command registry, sync hooks, credits matcher registry plus display and fetch gates, model selection hooks, `-ly-<N>` version stamp, hermetic test fixes | `options.tui.fork` (the store itself) |
| `toggle/ly-sidebar-minimalism` | base | `Toggle Compact Sidebar` command and `fork.IsSidebarMinimalism` | `options.tui.fork.sidebar_minimalism` (default `true`; legacy `options.tui.sidebar_minimalism`) |
| `toggle/ly-credits-display` | base | `Toggle Hypercredits` command; hides the balance and stops its fetches for every credits router | `options.tui.fork.show_credits` (default `true`; legacy `options.tui.show_hypercredits`) |
| `feat/ly-minimalist-sidebar` | `toggle/ly-sidebar-minimalism` | Sidebar hides disabled MCPs and never-active LSPs | reads `sidebar_minimalism` |
| `feat/ly-hypercredit-gateway-gate` | base | Credits matcher for gateway-served Hyper models | - |
| `feat/ly-small-model-display` | base | Small model shown below the main model (landing and sidebar), balance above both | - |
| `feat/ly-small-model-effort` | base | Selection hook: small model runs at its lowest reasoning effort | - |
| `feat/ly-apply-both` | base | `shift+enter` in the model dialog applies the model to both sizes | - |
| `feat/ly-symbol-key-hints` | base | Symbol rendering of key hints plus `Toggle Symbol Hints` | `options.tui.fork.symbol_hints` (default `true`; legacy `options.tui.symbol_hints`) |
| `feat/ly-code-mode-banner` | base | `CODE` badge and banner for the standard mode | - |
| `feat/ly-gateway-discovery-metadata` | base | Gateway `/v1/models` metadata mapped onto the catalog | - |
| `feat/ly-list-nogap` | base | `NoGapItem`: adjacent opted-in list items skip the gap row | - |
| `feat/ly-mcp-capability-counts` | base | One-line MCP capability counts in the LSP style | - |
| `docs/ly-fork-docs` | `upstream-main` | README fork section and this file | - |

## 4. The sync cycle

Run these steps whenever upstream moves. All of them are local until
the final push.

1. **Back up every branch.** Keep the backups forever, because they
   are the way back if a rebase goes wrong.

   ```sh
   day=$(date +%F)
   git fetch origin && git fetch upstream
   for b in $(git for-each-ref --format='%(refname:lstrip=3)' refs/remotes/origin); do
     [ "$b" = HEAD ] && continue
     git update-ref "refs/backups/pre-rebase-$day/$b" "origin/$b"
   done
   ```

   If a rebase drops code (for example because upstream rewrote what
   it touched), save the dropped hunk as a patch under the fork notes
   so it can be ported later.

2. **Fast-forward `upstream-main`.**

   ```sh
   git checkout upstream-main && git merge --ff-only upstream/main
   ```

3. **Rebase the base:**
   `git rebase upstream-main base/ly-fork-plumbing`. Resolve
   conflicts here first, because everything else sits on it.

4. **Rebase the toggles** onto the new base, using the old base tip as
   the cut point:

   ```sh
   git rebase --onto base/ly-fork-plumbing <old-base-tip> toggle/ly-sidebar-minimalism
   git rebase --onto base/ly-fork-plumbing <old-base-tip> toggle/ly-credits-display
   ```

5. **Rebase the features and docs.** Features on base use the same
   `--onto base <old-base-tip>` form. Features on a toggle rebase onto
   the new toggle tip, cut at the old one. `docs/ly-fork-docs` rebases
   onto `upstream-main`. Update the README tables and this file if
   anything changed.

6. **Test each branch on its own**, in a detached worktree so the main
   checkout stays free:

   ```sh
   git worktree add --detach /tmp/wt <branch>
   (cd /tmp/wt && go build ./... && go test ./...)
   git worktree remove /tmp/wt
   ```

   Also check that features build without their siblings, for example
   the small model display without the small model effort.

7. **Update `main` without rewriting it.** First add a sync merge
   whose tree is exactly `upstream-main`. Its parents are the old
   `main` and `upstream-main`, so history only grows:

   ```sh
   git checkout main
   sync=$(git commit-tree "upstream-main^{tree}" -p main -p upstream-main \
     -m "chore(sync): sync main to upstream-main $(git rev-parse --short=8 upstream-main)")
   git merge --ff-only "$sync"
   ```

   Then squash each branch in dependency order (base, toggles,
   features, docs), with one Conventional Commit each. Squash every
   branch **against its own parent branch**, not with a plain
   `git merge --squash`. A squash records no ancestry, so a plain
   `git merge --squash feat/x` would use `upstream-main` as the merge
   base. Any feature that edits lines base also edits (sidebar,
   elements, model info) would then conflict with base's own squash.
   Squashing against the parent applies only that branch's own
   changes:

   ```sh
   # squash <branch> <parent> <subject>
   squash() {
     tmp=$(git commit-tree "$1^{tree}" -p "$2" -m "$3")
     git cherry-pick --no-commit "$tmp"
     git commit -m "$3" -m "Squashed from $1 at $(git rev-parse --short "$1")."
   }
   squash base/ly-fork-plumbing upstream-main "feat(fork): add the fork plumbing for flags, commands and credits"
   squash toggle/ly-sidebar-minimalism base/ly-fork-plumbing "feat(fork): add the compact sidebar toggle"
   squash feat/ly-minimalist-sidebar toggle/ly-sidebar-minimalism "feat(ui): hide disabled MCPs and never-active LSPs from the sidebar"
   # ... one per branch; docs/ly-fork-docs squashes against upstream-main
   ```

   The result is the same as `git merge --squash` would produce if
   it knew the right base. `main` gets one commit per branch, and
   each branch keeps its granular commits. A squash never moves the
   branch itself.

8. **Test `main`:** `go build ./... && go test ./...`.

9. **Push.**
   - `upstream-main` and `main` are fast-forwards.
   - Rewritten branches use `--force-with-lease`.
   - `main` is never force-pushed.

   ```sh
   git push origin upstream-main main
   git push --force-with-lease origin base/ly-fork-plumbing toggle/ly-... feat/ly-... docs/ly-fork-docs
   ```

## 5. Conventions

- **Commits** follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):
  `type(scope): subject`, with a lowercase imperative subject, no
  trailing period, and a body that explains why when it helps. Types
  in use: `feat`, `fix`, `style`, `test`, `build`, `docs`, `chore`.
  Squash commits on `main` follow the same format.
- **Changelogs.** The fork keeps no separate changelog: this file and
  the README describe the fork. If a changelog is ever edited, it
  follows [Keep a Changelog 2.0.0](https://keepachangelog.com/en/2.0.0/):
  - an `Unreleased` section on top
  - `YYYY-MM-DD` dates
  - the change groups Added, Changed, Deprecated, Removed, Fixed and
    Security
  - `[YANKED]` releases
  - compare links at the bottom
- **Go.** Run `gofmt -w` on every touched file. Every fork behavior
  has a test in the branch that adds it.

## 6. Dropping or adding pieces

**Drop a feature.** Stop merging its branch into `main` at the next
sync. Because features never depend on each other, nothing else
changes. Keep the branch itself so the feature can come back later.

**Add a feature.**

1. Branch off `base/ly-fork-plumbing` (or off a toggle, if a toggle
   gates the feature). Name it `feat/ly-<name>`.
2. Put the logic in new files, and change upstream files only at the
   call sites.
3. Read flags through the toggle's helper. Register commands, sync
   hooks, credits matchers or selection hooks from `init()`.
4. Add tests, add a row to the README table and to the inventory
   above, and add the branch to the squash order in step 7.

**Add a toggle.**

1. Branch `toggle/ly-<name>` off base.
2. Add `internal/fork/<name>.go` with:
   - the flag name constant
   - an `Is<Name>(cfg)` helper built on `cfg.ForkFlag`
   - an `init()` that registers the `Toggle ...` command (and a
     credits display gate or sync hook if needed)
3. If the switch replaces an old flat key, add the key to
   `legacyForkFlags` in base.
4. Features that the toggle gates branch off the toggle and only call
   the helper.

**Drop a toggle.** Drop the features that branch off it first, or
move them onto base and replace the helper with a constant.

**Add a credits router.** Write a feature branch on base with a file
that calls `hyper.RegisterCreditsMatcher` from `init()`. The display,
the fetch gate and `toggle/ly-credits-display` pick it up
automatically.

**Restore dropped code.** Patches dropped during a rework are kept
next to the backup refs (the fork notes directory and
`refs/backups/pre-rebase-<date>/<branch>`). To port one onto the new
base, run `git show <backup-ref>:<path>`, or apply the patch and
resolve it against the new upstream.
