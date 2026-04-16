export const PRE_EXEC_TARGET_MODE_TASK_PANES = "task_panes" as const;
export const PRE_EXEC_TARGET_MODE_ALL_PANES = "all_panes" as const;

export const PRE_EXEC_TARGET_MODES = [
    PRE_EXEC_TARGET_MODE_TASK_PANES,
    PRE_EXEC_TARGET_MODE_ALL_PANES,
] as const;

export type PreExecTargetModeValue = typeof PRE_EXEC_TARGET_MODES[number];
