export namespace config {
	
	export class AgentModelOverride {
	    name: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentModelOverride(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.model = source["model"];
	    }
	}
	export class AgentModel {
	    from: string;
	    to: string;
	    overrides?: AgentModelOverride[];
	
	    static createFrom(source: any = {}) {
	        return new AgentModel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.overrides = this.convertValues(source["overrides"], AgentModelOverride);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ClaudeEnvConfig {
	    default_enabled: boolean;
	    vars?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ClaudeEnvConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.default_enabled = source["default_enabled"];
	        this.vars = source["vars"];
	    }
	}
	export class WorktreeConfig {
	    enabled: boolean;
	    force_cleanup: boolean;
	    setup_scripts: string[];
	    copy_files: string[];
	    copy_dirs: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorktreeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.force_cleanup = source["force_cleanup"];
	        this.setup_scripts = source["setup_scripts"];
	        this.copy_files = source["copy_files"];
	        this.copy_dirs = source["copy_dirs"];
	    }
	}
	export class Config {
	    shell: string;
	    prefix: string;
	    keys: Record<string, string>;
	    quake_mode: boolean;
	    global_hotkey: string;
	    worktree: WorktreeConfig;
	    agent_model?: AgentModel;
	    pane_env?: Record<string, string>;
	    pane_env_default_enabled: boolean;
	    claude_env?: ClaudeEnvConfig;
	    websocket_port: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shell = source["shell"];
	        this.prefix = source["prefix"];
	        this.keys = source["keys"];
	        this.quake_mode = source["quake_mode"];
	        this.global_hotkey = source["global_hotkey"];
	        this.worktree = this.convertValues(source["worktree"], WorktreeConfig);
	        this.agent_model = this.convertValues(source["agent_model"], AgentModel);
	        this.pane_env = source["pane_env"];
	        this.pane_env_default_enabled = source["pane_env_default_enabled"];
	        this.claude_env = this.convertValues(source["claude_env"], ClaudeEnvConfig);
	        this.websocket_port = source["websocket_port"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace git {
	
	export class WorktreeInfo {
	    path: string;
	    branch: string;
	    isMain: boolean;
	    isDetached: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.branch = source["branch"];
	        this.isMain = source["isMain"];
	        this.isDetached = source["isDetached"];
	    }
	}

}

export namespace install {
	
	export class ShimInstallResult {
	    installed_path: string;
	    path_updated: boolean;
	    restart_needed: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ShimInstallResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed_path = source["installed_path"];
	        this.path_updated = source["path_updated"];
	        this.restart_needed = source["restart_needed"];
	        this.message = source["message"];
	    }
	}

}

export namespace main {
	
	export class CreateSessionOptions {
	    enable_agent_team: boolean;
	    use_claude_env: boolean;
	    use_pane_env: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CreateSessionOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enable_agent_team = source["enable_agent_team"];
	        this.use_claude_env = source["use_claude_env"];
	        this.use_pane_env = source["use_pane_env"];
	    }
	}
	export class FileContent {
	    path: string;
	    content: string;
	    line_count: number;
	    size: number;
	    truncated: boolean;
	    binary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	        this.line_count = source["line_count"];
	        this.size = source["size"];
	        this.truncated = source["truncated"];
	        this.binary = source["binary"];
	    }
	}
	export class FileEntry {
	    name: string;
	    path: string;
	    is_dir: boolean;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.is_dir = source["is_dir"];
	        this.size = source["size"];
	    }
	}
	export class GitGraphCommit {
	    hash: string;
	    full_hash: string;
	    parents: string[];
	    subject: string;
	    author_name: string;
	    author_date: string;
	    refs: string[];
	
	    static createFrom(source: any = {}) {
	        return new GitGraphCommit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.full_hash = source["full_hash"];
	        this.parents = source["parents"];
	        this.subject = source["subject"];
	        this.author_name = source["author_name"];
	        this.author_date = source["author_date"];
	        this.refs = source["refs"];
	    }
	}
	export class GitStatusResult {
	    branch: string;
	    modified: string[];
	    staged: string[];
	    untracked: string[];
	    ahead: number;
	    behind: number;
	
	    static createFrom(source: any = {}) {
	        return new GitStatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.modified = source["modified"];
	        this.staged = source["staged"];
	        this.untracked = source["untracked"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	    }
	}
	export class SessionLogEntry {
	    seq: number;
	    ts: string;
	    level: string;
	    msg: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seq = source["seq"];
	        this.ts = source["ts"];
	        this.level = source["level"];
	        this.msg = source["msg"];
	        this.source = source["source"];
	    }
	}
	export class ValidationRules {
	    min_override_name_len: number;
	
	    static createFrom(source: any = {}) {
	        return new ValidationRules(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.min_override_name_len = source["min_override_name_len"];
	    }
	}
	export class WorkingDiffFile {
	    path: string;
	    old_path: string;
	    status: string;
	    additions: number;
	    deletions: number;
	    diff: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkingDiffFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.old_path = source["old_path"];
	        this.status = source["status"];
	        this.additions = source["additions"];
	        this.deletions = source["deletions"];
	        this.diff = source["diff"];
	    }
	}
	export class WorkingDiffResult {
	    files: WorkingDiffFile[];
	    total_added: number;
	    total_deleted: number;
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkingDiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], WorkingDiffFile);
	        this.total_added = source["total_added"];
	        this.total_deleted = source["total_deleted"];
	        this.truncated = source["truncated"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WorktreeSessionOptions {
	    branch_name: string;
	    base_branch: string;
	    pull_before_create: boolean;
	    enable_agent_team: boolean;
	    use_claude_env: boolean;
	    use_pane_env: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeSessionOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch_name = source["branch_name"];
	        this.base_branch = source["base_branch"];
	        this.pull_before_create = source["pull_before_create"];
	        this.enable_agent_team = source["enable_agent_team"];
	        this.use_claude_env = source["use_claude_env"];
	        this.use_pane_env = source["use_pane_env"];
	    }
	}
	export class WorktreeStatus {
	    has_worktree: boolean;
	    has_uncommitted: boolean;
	    has_unpushed: boolean;
	    branch_name: string;
	    is_detached: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.has_worktree = source["has_worktree"];
	        this.has_uncommitted = source["has_uncommitted"];
	        this.has_unpushed = source["has_unpushed"];
	        this.branch_name = source["branch_name"];
	        this.is_detached = source["is_detached"];
	    }
	}

}

export namespace tmux {
	
	export class LayoutNode {
	    type: string;
	    direction?: string;
	    ratio?: number;
	    pane_id: number;
	    children?: LayoutNode[];
	
	    static createFrom(source: any = {}) {
	        return new LayoutNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.direction = source["direction"];
	        this.ratio = source["ratio"];
	        this.pane_id = source["pane_id"];
	        this.children = this.convertValues(source["children"], LayoutNode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaneSnapshot {
	    id: string;
	    index: number;
	    title?: string;
	    active: boolean;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new PaneSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.index = source["index"];
	        this.title = source["title"];
	        this.active = source["active"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class SessionWorktreeInfo {
	    path?: string;
	    repo_path?: string;
	    branch_name?: string;
	    base_branch?: string;
	    is_detached: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SessionWorktreeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.repo_path = source["repo_path"];
	        this.branch_name = source["branch_name"];
	        this.base_branch = source["base_branch"];
	        this.is_detached = source["is_detached"];
	    }
	}
	export class WindowSnapshot {
	    id: number;
	    name: string;
	    layout?: LayoutNode;
	    active_pane: number;
	    panes: PaneSnapshot[];
	
	    static createFrom(source: any = {}) {
	        return new WindowSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.layout = this.convertValues(source["layout"], LayoutNode);
	        this.active_pane = source["active_pane"];
	        this.panes = this.convertValues(source["panes"], PaneSnapshot);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionSnapshot {
	    id: number;
	    name: string;
	    // Go type: time
	    created_at: any;
	    is_idle: boolean;
	    active_window_id: number;
	    is_agent_team?: boolean;
	    windows: WindowSnapshot[];
	    worktree?: SessionWorktreeInfo;
	    root_path?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.is_idle = source["is_idle"];
	        this.active_window_id = source["active_window_id"];
	        this.is_agent_team = source["is_agent_team"];
	        this.windows = this.convertValues(source["windows"], WindowSnapshot);
	        this.worktree = this.convertValues(source["worktree"], SessionWorktreeInfo);
	        this.root_path = source["root_path"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

