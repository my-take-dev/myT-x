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
	export class MCPServerConfigParam {
	    key: string;
	    label: string;
	    default_value: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerConfigParam(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.default_value = source["default_value"];
	        this.description = source["description"];
	    }
	}
	export class MCPServerConfig {
	    id: string;
	    name: string;
	    description?: string;
	    command: string;
	    args?: string[];
	    env?: Record<string, string>;
	    enabled: boolean;
	    usage_sample?: string;
	    config_params?: MCPServerConfigParam[];
	
	    static createFrom(source: any = {}) {
	        return new MCPServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.env = source["env"];
	        this.enabled = source["enabled"];
	        this.usage_sample = source["usage_sample"];
	        this.config_params = this.convertValues(source["config_params"], MCPServerConfigParam);
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
	    viewer_shortcuts?: Record<string, string>;
	    viewer_sidebar_mode?: string;
	    default_session_dir?: string;
	    mcp_servers?: MCPServerConfig[];
	    chat_overlay_percentage?: number;
	
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
	        this.viewer_shortcuts = source["viewer_shortcuts"];
	        this.viewer_sidebar_mode = source["viewer_sidebar_mode"];
	        this.default_session_dir = source["default_session_dir"];
	        this.mcp_servers = this.convertValues(source["mcp_servers"], MCPServerConfig);
	        this.chat_overlay_percentage = source["chat_overlay_percentage"];
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

export namespace devpanel {
	
	export class CommitResult {
	    hash: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CommitResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.message = source["message"];
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
	    conflicted: string[];
	    ahead: number;
	    behind: number;
	    upstream_configured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GitStatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.modified = source["modified"];
	        this.staged = source["staged"];
	        this.untracked = source["untracked"];
	        this.conflicted = source["conflicted"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.upstream_configured = source["upstream_configured"];
	    }
	}
	export class PullResult {
	    updated: boolean;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new PullResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updated = source["updated"];
	        this.summary = source["summary"];
	    }
	}
	export class PushResult {
	    remote_name: string;
	    branch_name: string;
	    upstream_set: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PushResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote_name = source["remote_name"];
	        this.branch_name = source["branch_name"];
	        this.upstream_set = source["upstream_set"];
	    }
	}
	export class SearchContentLine {
	    line: number;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchContentLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.content = source["content"];
	    }
	}
	export class SearchFileResult {
	    path: string;
	    name: string;
	    is_name_match: boolean;
	    content_lines: SearchContentLine[];
	
	    static createFrom(source: any = {}) {
	        return new SearchFileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.is_name_match = source["is_name_match"];
	        this.content_lines = this.convertValues(source["content_lines"], SearchContentLine);
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

}

export namespace git {
	
	export class WorktreeHealth {
	    isHealthy: boolean;
	    issues?: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorktreeHealth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isHealthy = source["isHealthy"];
	        this.issues = source["issues"];
	    }
	}
	export class WorktreeInfo {
	    path: string;
	    branch: string;
	    isMain: boolean;
	    isDetached: boolean;
	    health?: WorktreeHealth;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.branch = source["branch"];
	        this.isMain = source["isMain"];
	        this.isDetached = source["isDetached"];
	        this.health = this.convertValues(source["health"], WorktreeHealth);
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

export namespace inputhistory {
	
	export class Entry {
	    seq: number;
	    ts: string;
	    pane_id: string;
	    input: string;
	    source: string;
	    session: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seq = source["seq"];
	        this.ts = source["ts"];
	        this.pane_id = source["pane_id"];
	        this.input = source["input"];
	        this.source = source["source"];
	        this.session = source["session"];
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

export namespace ipc {
	
	export class MCPStdioResolvePayload {
	    session_name: string;
	    mcp_id: string;
	    pipe_path: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPStdioResolvePayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session_name = source["session_name"];
	        this.mcp_id = source["mcp_id"];
	        this.pipe_path = source["pipe_path"];
	    }
	}

}

export namespace main {
	
	export class CreateSessionOptions {
	    enable_agent_team: boolean;
	    use_claude_env: boolean;
	    use_pane_env: boolean;
	    use_session_pane_scope: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CreateSessionOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enable_agent_team = source["enable_agent_team"];
	        this.use_claude_env = source["use_claude_env"];
	        this.use_pane_env = source["use_pane_env"];
	        this.use_session_pane_scope = source["use_session_pane_scope"];
	    }
	}
	export class OrchestratorAgent {
	    name: string;
	    pane_id: string;
	    role: string;
	
	    static createFrom(source: any = {}) {
	        return new OrchestratorAgent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.pane_id = source["pane_id"];
	        this.role = source["role"];
	    }
	}
	export class OrchestratorTask {
	    task_id: string;
	    agent_name: string;
	    assignee_pane_id: string;
	    sender_pane_id: string;
	    sender_name: string;
	    status: string;
	    sent_at: string;
	    completed_at: string;
	    message_preview: string;
	    response_preview: string;
	
	    static createFrom(source: any = {}) {
	        return new OrchestratorTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task_id = source["task_id"];
	        this.agent_name = source["agent_name"];
	        this.assignee_pane_id = source["assignee_pane_id"];
	        this.sender_pane_id = source["sender_pane_id"];
	        this.sender_name = source["sender_name"];
	        this.status = source["status"];
	        this.sent_at = source["sent_at"];
	        this.completed_at = source["completed_at"];
	        this.message_preview = source["message_preview"];
	        this.response_preview = source["response_preview"];
	    }
	}
	export class OrchestratorTaskDetail {
	    task_id: string;
	    agent_name: string;
	    sender_name: string;
	    status: string;
	    sent_at: string;
	    completed_at: string;
	    message_content: string;
	    response_content: string;
	
	    static createFrom(source: any = {}) {
	        return new OrchestratorTaskDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task_id = source["task_id"];
	        this.agent_name = source["agent_name"];
	        this.sender_name = source["sender_name"];
	        this.status = source["status"];
	        this.sent_at = source["sent_at"];
	        this.completed_at = source["completed_at"];
	        this.message_content = source["message_content"];
	        this.response_content = source["response_content"];
	    }
	}
	export class PaneProcessStatus {
	    pane_id: string;
	    has_child_process: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PaneProcessStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pane_id = source["pane_id"];
	        this.has_child_process = source["has_child_process"];
	    }
	}
	export class TaskSchedulerOrchestratorReadiness {
	    ready: boolean;
	    db_exists: boolean;
	    agent_count: number;
	    has_panes: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TaskSchedulerOrchestratorReadiness(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ready = source["ready"];
	        this.db_exists = source["db_exists"];
	        this.agent_count = source["agent_count"];
	        this.has_panes = source["has_panes"];
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

}

export namespace mcp {
	
	export class ConfigParam {
	    key: string;
	    label: string;
	    default_value: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigParam(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.default_value = source["default_value"];
	        this.description = source["description"];
	    }
	}
	export class Snapshot {
	    id: string;
	    name: string;
	    description: string;
	    enabled: boolean;
	    status: string;
	    error?: string;
	    usage_sample?: string;
	    config_params?: ConfigParam[];
	    pipe_path?: string;
	    bridge_command?: string;
	    bridge_args?: string[];
	    kind?: string;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.enabled = source["enabled"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.usage_sample = source["usage_sample"];
	        this.config_params = this.convertValues(source["config_params"], ConfigParam);
	        this.pipe_path = source["pipe_path"];
	        this.bridge_command = source["bridge_command"];
	        this.bridge_args = source["bridge_args"];
	        this.kind = source["kind"];
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

export namespace orchestrator {
	
	export class TeamMemberSkill {
	    name: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new TeamMemberSkill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	    }
	}
	export class TeamMember {
	    id: string;
	    team_id: string;
	    order: number;
	    pane_title: string;
	    role: string;
	    command: string;
	    args: string[];
	    custom_message: string;
	    skills?: TeamMemberSkill[];
	
	    static createFrom(source: any = {}) {
	        return new TeamMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.team_id = source["team_id"];
	        this.order = source["order"];
	        this.pane_title = source["pane_title"];
	        this.role = source["role"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.custom_message = source["custom_message"];
	        this.skills = this.convertValues(source["skills"], TeamMemberSkill);
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
	export class BootstrapMemberToPaneRequest {
	    pane_id: string;
	    pane_state: string;
	    team_name: string;
	    member: TeamMember;
	    bootstrap_delay_ms: number;
	    session_name: string;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapMemberToPaneRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pane_id = source["pane_id"];
	        this.pane_state = source["pane_state"];
	        this.team_name = source["team_name"];
	        this.member = this.convertValues(source["member"], TeamMember);
	        this.bootstrap_delay_ms = source["bootstrap_delay_ms"];
	        this.session_name = source["session_name"];
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
	export class BootstrapMemberToPaneResult {
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new BootstrapMemberToPaneResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.warnings = source["warnings"];
	    }
	}
	export class StartTeamRequest {
	    team_id: string;
	    launch_mode: string;
	    source_session_name: string;
	    new_session_name: string;
	
	    static createFrom(source: any = {}) {
	        return new StartTeamRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.team_id = source["team_id"];
	        this.launch_mode = source["launch_mode"];
	        this.source_session_name = source["source_session_name"];
	        this.new_session_name = source["new_session_name"];
	    }
	}
	export class StartTeamResult {
	    session_name: string;
	    launch_mode: string;
	    member_pane_ids: Record<string, string>;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new StartTeamResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session_name = source["session_name"];
	        this.launch_mode = source["launch_mode"];
	        this.member_pane_ids = source["member_pane_ids"];
	        this.warnings = source["warnings"];
	    }
	}
	export class TeamDefinition {
	    id: string;
	    name: string;
	    description?: string;
	    order: number;
	    bootstrap_delay_ms?: number;
	    storage_location?: string;
	    members: TeamMember[];
	
	    static createFrom(source: any = {}) {
	        return new TeamDefinition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.order = source["order"];
	        this.bootstrap_delay_ms = source["bootstrap_delay_ms"];
	        this.storage_location = source["storage_location"];
	        this.members = this.convertValues(source["members"], TeamMember);
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

export namespace scheduler {
	
	export class EntryStatus {
	    id: string;
	    title: string;
	    pane_id: string;
	    message: string;
	    interval_seconds: number;
	    max_count: number;
	    current_count: number;
	    running: boolean;
	    stop_reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new EntryStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.pane_id = source["pane_id"];
	        this.message = source["message"];
	        this.interval_seconds = source["interval_seconds"];
	        this.max_count = source["max_count"];
	        this.current_count = source["current_count"];
	        this.running = source["running"];
	        this.stop_reason = source["stop_reason"];
	    }
	}
	export class Template {
	    title: string;
	    message: string;
	    interval_seconds: number;
	    max_count: number;
	
	    static createFrom(source: any = {}) {
	        return new Template(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.message = source["message"];
	        this.interval_seconds = source["interval_seconds"];
	        this.max_count = source["max_count"];
	    }
	}

}

export namespace sessionlog {
	
	export class Entry {
	    seq: number;
	    ts: string;
	    level: string;
	    msg: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
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

}

export namespace taskscheduler {
	
	export class QueueConfig {
	    pre_exec_enabled: boolean;
	    pre_exec_target_mode: string;
	    pre_exec_reset_delay_s: number;
	    pre_exec_idle_timeout_s: number;
	
	    static createFrom(source: any = {}) {
	        return new QueueConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pre_exec_enabled = source["pre_exec_enabled"];
	        this.pre_exec_target_mode = source["pre_exec_target_mode"];
	        this.pre_exec_reset_delay_s = source["pre_exec_reset_delay_s"];
	        this.pre_exec_idle_timeout_s = source["pre_exec_idle_timeout_s"];
	    }
	}
	export class QueueItem {
	    id: string;
	    title: string;
	    message: string;
	    target_pane_id: string;
	    order_index: number;
	    status: string;
	    orc_task_id?: string;
	    created_at: string;
	    started_at?: string;
	    completed_at?: string;
	    error_message?: string;
	    clear_before: boolean;
	    clear_command?: string;
	
	    static createFrom(source: any = {}) {
	        return new QueueItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.message = source["message"];
	        this.target_pane_id = source["target_pane_id"];
	        this.order_index = source["order_index"];
	        this.status = source["status"];
	        this.orc_task_id = source["orc_task_id"];
	        this.created_at = source["created_at"];
	        this.started_at = source["started_at"];
	        this.completed_at = source["completed_at"];
	        this.error_message = source["error_message"];
	        this.clear_before = source["clear_before"];
	        this.clear_command = source["clear_command"];
	    }
	}
	export class QueueStatus {
	    config: QueueConfig;
	    items: QueueItem[];
	    run_status: string;
	    current_index: number;
	    session_name: string;
	    pre_exec_progress?: string;
	
	    static createFrom(source: any = {}) {
	        return new QueueStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], QueueConfig);
	        this.items = this.convertValues(source["items"], QueueItem);
	        this.run_status = source["run_status"];
	        this.current_index = source["current_index"];
	        this.session_name = source["session_name"];
	        this.pre_exec_progress = source["pre_exec_progress"];
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

export namespace worktree {
	
	export class OrphanedWorktree {
	    path: string;
	    branchName: string;
	    hasChanges: boolean;
	    health?: git.WorktreeHealth;
	
	    static createFrom(source: any = {}) {
	        return new OrphanedWorktree(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.branchName = source["branchName"];
	        this.hasChanges = source["hasChanges"];
	        this.health = this.convertValues(source["health"], git.WorktreeHealth);
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
	    use_session_pane_scope: boolean;
	
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
	        this.use_session_pane_scope = source["use_session_pane_scope"];
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

