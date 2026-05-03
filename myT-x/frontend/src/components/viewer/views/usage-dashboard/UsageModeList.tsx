import {useId} from "react";
import type {UsageDashboardSelection} from "./useUsageDashboard";
import {useUsageDashboardI18n} from "./i18n";

interface UsageModeListOption {
    readonly id: UsageDashboardSelection;
    readonly label: string;
}

interface UsageModeListProps {
    readonly selection: UsageDashboardSelection;
    readonly onSelectionChange: (selection: UsageDashboardSelection) => void;
    readonly options: ReadonlyArray<UsageModeListOption>;
}

export function UsageModeList({selection, onSelectionChange, options}: UsageModeListProps) {
    const tr = useUsageDashboardI18n();
    const labelId = useId();
    return (
        <label className="usage-dashboard-mode-list">
            <span id={labelId} className="usage-dashboard-mode-list-label">
                {tr("viewer.usageDashboard.modeListLabel", "表示", "View")}
            </span>
            <select
                className="usage-dashboard-mode-select"
                aria-labelledby={labelId}
                value={selection}
                onChange={(event) => {
                    const selectedOption = options.find((option) => option.id === event.currentTarget.value);
                    if (selectedOption) {
                        onSelectionChange(selectedOption.id);
                        return;
                    }
                    if (import.meta.env.DEV) {
                        console.warn("[UsageDashboard] Ignoring unknown dashboard selection", {
                            value: event.currentTarget.value,
                        });
                    }
                }}
            >
                {options.map((option) => (
                    <option key={option.id} value={option.id}>
                        {option.label}
                    </option>
                ))}
            </select>
        </label>
    );
}
