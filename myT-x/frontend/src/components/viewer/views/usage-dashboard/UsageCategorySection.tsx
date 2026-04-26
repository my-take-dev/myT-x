import {useMemo, useState} from "react";
import type {usagedashboard} from "../../../../../wailsjs/go/models";
import {ItemDailyUsageChart} from "./ItemDailyUsageChart";
import {RankingTable} from "./RankingTable";
import {useUsageDashboardI18n} from "./i18n";

type CategoryView = "ranking" | "daily";
export type UsageCategoryKey = "skills" | "agents" | "slash";

const ALL_ITEMS_SELECTION = "all";

export interface UsageCategory<K extends UsageCategoryKey = UsageCategoryKey> {
    readonly key: K;
    readonly label: string;
    readonly ranking: ReadonlyArray<usagedashboard.UsageEntry>;
    readonly dailySeries: ReadonlyArray<usagedashboard.DailyUsageSeries>;
    readonly emptyLabel: string;
}

interface UsageCategorySectionProps<K extends UsageCategoryKey> {
    readonly categories: ReadonlyArray<UsageCategory<K>>;
    readonly initialCategory: K;
    readonly ariaLabel: string;
    readonly colorVar: `--${string}`;
}

export function UsageCategorySection<K extends UsageCategoryKey>(props: UsageCategorySectionProps<K>) {
    const {categories, initialCategory, ariaLabel, colorVar} = props;
    const tr = useUsageDashboardI18n();
    const [categoryKey, setCategoryKey] = useState<K>(initialCategory);
    const [view, setView] = useState<CategoryView>("ranking");
    const [selectedByCategory, setSelectedByCategory] = useState<Partial<Record<K, string>>>({});

    const category = useMemo(
        () => categories.find((candidate) => candidate.key === categoryKey) ?? categories[0],
        [categories, categoryKey],
    );
    if (!category) {
        return null;
    }
    const dailySeries = alignDailySeriesToRanking(category.dailySeries, category.ranking);
    const selection = resolveSelection(dailySeries, selectedByCategory[category.key]);

    return (
        <section className="usage-dashboard-section">
            <div className="usage-dashboard-category-toolbar">
                <div
                    className="usage-dashboard-subtabs"
                    role="tablist"
                    aria-label={ariaLabel}
                >
                    {categories.map((candidate) => (
                        <CategoryTab
                            key={candidate.key}
                            current={category.key}
                            target={candidate.key}
                            label={`${candidate.label} (${categoryItemCount(candidate)})`}
                            onSelect={setCategoryKey}
                        />
                    ))}
                </div>
                <div
                    className="usage-dashboard-view-toggle"
                    role="tablist"
                    aria-label={tr("viewer.usageDashboard.categoryView", "表示形式", "Category view")}
                >
                    <ViewToggle current={view} target="ranking" label={tr("viewer.usageDashboard.rank30Days", "30日統計", "30-day stats")} onSelect={setView}/>
                    <ViewToggle current={view} target="daily" label={tr("viewer.usageDashboard.daily", "日別", "Daily")} onSelect={setView}/>
                </div>
            </div>
            {view === "ranking" ? (
                <RankingTable
                    entries={category.ranking}
                    emptyLabel={category.emptyLabel}
                    ariaLabel={ariaLabel}
                />
            ) : (
                <div className="usage-dashboard-daily-item">
                    {dailySeries.length > 0 ? (
                        <>
                            <label className="usage-dashboard-item-select-label">
                                <span>{tr("viewer.usageDashboard.item", "対象", "Item")}</span>
                                <select
                                    className="usage-dashboard-item-select"
                                    value={selection}
                                    aria-label={tr("viewer.usageDashboard.itemSelector", "日別グラフ対象", "Daily graph item")}
                                    onChange={(event) => {
                                        const nextSelection = event.currentTarget.value;
                                        setSelectedByCategory((current) => ({
                                            ...current,
                                            [category.key]: nextSelection,
                                        }));
                                    }}
                                >
                                    <option value={ALL_ITEMS_SELECTION}>
                                        {tr("viewer.usageDashboard.allItems", "all", "all")}
                                    </option>
                                    {dailySeries.map((series) => (
                                        <option key={series.name} value={itemSelectionValue(series.name)}>
                                            {series.name} ({series.total_count})
                                        </option>
                                    ))}
                                </select>
                            </label>
                            <ItemDailyUsageChart
                                dailySeries={dailySeries}
                                selection={selection}
                                labelCount={tr("viewer.usageDashboard.uses", "利用回数", "Uses")}
                                labelTotal={tr("viewer.usageDashboard.total", "合計", "Total")}
                                labelOther={tr("viewer.usageDashboard.other", "その他", "Other")}
                                colorVar={colorVar}
                            />
                        </>
                    ) : (
                        <div className="usage-dashboard-ranking-empty">{category.emptyLabel}</div>
                    )}
                </div>
            )}
        </section>
    );
}

function categoryItemCount(category: UsageCategory): string {
    return String(category.ranking.length);
}

function resolveSelection(
    dailySeries: ReadonlyArray<usagedashboard.DailyUsageSeries>,
    selection: string | undefined,
): string {
    if (!selection || selection === ALL_ITEMS_SELECTION) {
        return ALL_ITEMS_SELECTION;
    }
    return dailySeries.some((series) => itemSelectionValue(series.name) === selection)
        ? selection
        : ALL_ITEMS_SELECTION;
}

function alignDailySeriesToRanking(
    dailySeries: ReadonlyArray<usagedashboard.DailyUsageSeries>,
    ranking: ReadonlyArray<usagedashboard.UsageEntry>,
): ReadonlyArray<usagedashboard.DailyUsageSeries> {
    if (dailySeries.length === 0 || ranking.length === 0) {
        return [];
    }
    const dailyByName = new Map(dailySeries.map((series) => [series.name, series]));
    return ranking
        .map((entry) => dailyByName.get(entry.name))
        .filter((series): series is usagedashboard.DailyUsageSeries => series !== undefined);
}

function itemSelectionValue(name: string): string {
    return `item:${encodeURIComponent(name)}`;
}

interface CategoryTabProps<K extends UsageCategoryKey> {
    readonly current: K;
    readonly target: K;
    readonly label: string;
    readonly onSelect: (next: K) => void;
}

function CategoryTab<K extends UsageCategoryKey>({current, target, label, onSelect}: CategoryTabProps<K>) {
    const selected = current === target;
    return (
        <button
            type="button"
            role="tab"
            aria-selected={selected}
            className="usage-dashboard-subtab"
            onClick={() => {
                if (!selected) onSelect(target);
            }}
        >
            {label}
        </button>
    );
}

interface ViewToggleProps {
    readonly current: CategoryView;
    readonly target: CategoryView;
    readonly label: string;
    readonly onSelect: (next: CategoryView) => void;
}

function ViewToggle({current, target, label, onSelect}: ViewToggleProps) {
    const selected = current === target;
    return (
        <button
            type="button"
            role="tab"
            aria-selected={selected}
            className="usage-dashboard-view-toggle-btn"
            onClick={() => {
                if (!selected) onSelect(target);
            }}
        >
            {label}
        </button>
    );
}
