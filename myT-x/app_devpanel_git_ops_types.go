package main

// DevPanelCommitResult represents the result of a git commit operation.
type DevPanelCommitResult struct {
	Hash    string `json:"hash"`    // short commit hash
	Message string `json:"message"` // first line of commit message
}

// DevPanelPushResult represents the result of a git push operation.
type DevPanelPushResult struct {
	RemoteName  string `json:"remote_name"`  // e.g. "origin"
	BranchName  string `json:"branch_name"`  // e.g. "main"
	UpstreamSet bool   `json:"upstream_set"` // true if --set-upstream was used
}

// DevPanelPullResult represents the result of a git pull operation.
type DevPanelPullResult struct {
	Updated bool   `json:"updated"` // true if any changes were pulled
	Summary string `json:"summary"` // human-readable summary
}
