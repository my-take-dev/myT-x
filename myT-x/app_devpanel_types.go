package main

import "myT-x/internal/devpanel"

// Type aliases for Wails binding compatibility.
// Wails generates bindings from App method signatures in the main package.
// These aliases re-export internal devpanel types so that Wails can
// discover them without exposing the internal package directly.
type FileEntry = devpanel.FileEntry
type FileContent = devpanel.FileContent
type GitGraphCommit = devpanel.GitGraphCommit
type GitStatusResult = devpanel.GitStatusResult

// WorkingDiffStatus is a type alias (= string) re-exported from devpanel.
// Wails emits it as a plain string in TypeScript bindings.
type WorkingDiffStatus = devpanel.WorkingDiffStatus

type WorkingDiffFile = devpanel.WorkingDiffFile
type WorkingDiffResult = devpanel.WorkingDiffResult
type SearchFileResult = devpanel.SearchFileResult
type SearchContentLine = devpanel.SearchContentLine
