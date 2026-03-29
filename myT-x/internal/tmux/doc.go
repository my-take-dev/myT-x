// Package tmux implements a tmux-compatible session/window/pane multiplexer
// backed by ConPTY terminals on Windows.
//
// # Architecture
//
// The package is organised around two core components:
//
//   - SessionManager: owns all session/window/pane state, protected by a single
//     RWMutex. Methods follow the "Locked"/"RLocked" suffix convention when they
//     require the caller to hold the lock.
//
//   - CommandRouter: dispatches tmux-compatible IPC requests (new-session,
//     send-keys, list-panes, etc.) to handler functions, which delegate state
//     mutations to SessionManager.
//
// # File layout
//
// Types & models:
//
//	types.go                             — Model types (TmuxSession, TmuxWindow, TmuxPane, snapshots, events)
//	layout.go                            — Pane layout tree (LayoutNode, split/swap/clone)
//	buffer_store.go                      — Paste buffer storage (BufferStore, PasteBuffer)
//	pane_output_history.go               — Ring buffer for terminal output capture
//
// SessionManager (state management):
//
//	session_manager.go                   — Struct, constructor, mutation markers
//	session_manager_sessions.go          — Session CRUD
//	session_manager_windows.go           — Window CRUD
//	session_manager_panes.go             — Pane split, activation, layout presets
//	session_manager_pane_lifecycle.go    — Pane lifecycle (creation, destruction, swap)
//	session_manager_pane_io.go           — Pane I/O (list, write, resize, rename)
//	session_manager_snapshot.go          — Snapshot generation and caching
//	session_manager_targets.go           — Target resolution, directional navigation
//	session_manager_env.go               — Environment variable management
//	session_manager_idle.go              — Idle state detection
//	session_manager_helpers.go           — Shared utilities (parsePaneID, SanitizeSessionName, etc.)
//
// CommandRouter (request dispatch):
//
//	command_router.go                    — Struct, constructor, env resolution, Execute, target-resolve helpers
//	command_router_helpers.go            — Type coercion utilities (mustBool, okResp, etc.)
//	command_router_terminal.go           — Terminal attachment, env merging, panic recovery
//	command_router_sendkeys.go           — send-keys payload writing (typewriter, CRLF modes)
//
// Command handlers (one file per command family):
//
//	command_router_handlers_session.go   — new/kill/rename/list/has/attach-session
//	command_router_handlers_window.go    — new/kill/rename/list/select/activate-window
//	command_router_handlers_pane.go      — split-window, select/kill/resize-pane, capture-pane, copy-mode
//	command_router_handlers_display.go   — display-message
//	command_router_handlers_buffer.go    — list/set/paste/load/save-buffer
//	command_router_handlers_shell.go     — run-shell, if-shell
//	command_router_handlers_mcp.go       — mcp-resolve-stdio, resolve-session-by-cwd
//
// Parsing & formatting:
//
//	format.go                            — tmux #{var} format string expansion
//	key_table.go                         — send-keys / copy-mode key translation tables
//	tmux_command_parser.go               — CLI argument parsing
package tmux
