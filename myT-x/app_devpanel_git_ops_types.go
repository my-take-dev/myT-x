package main

import "myT-x/internal/devpanel"

// Type aliases for Wails binding compatibility.
// Wails generates bindings from App method signatures in the main package.
// These aliases re-export internal devpanel types so that Wails can
// discover them without exposing the internal package directly.
type DevPanelCommitResult = devpanel.CommitResult
type DevPanelPushResult = devpanel.PushResult
type DevPanelPullResult = devpanel.PullResult
