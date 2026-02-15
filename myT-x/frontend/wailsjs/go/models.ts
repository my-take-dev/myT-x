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
	
	export class WorktreeConfig {
	    enabled: boolean;
	    force_cleanup: boolean;
	    setup_scripts: string[];
	    copy_files: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorktreeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.force_cleanup = source["force_cleanup"];
	        this.setup_scripts = source["setup_scripts"];
	        this.copy_files = source["copy_files"];
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
	export class WorktreeSessionOptions {
	    branch_name: string;
	    base_branch: string;
	    pull_before_create: boolean;
	    enable_agent_team: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeSessionOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch_name = source["branch_name"];
	        this.base_branch = source["base_branch"];
	        this.pull_before_create = source["pull_before_create"];
	        this.enable_agent_team = source["enable_agent_team"];
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

