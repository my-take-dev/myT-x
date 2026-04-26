import {useId, useMemo, type MouseEvent, type PointerEvent, type WheelEvent} from "react";
import {
    Bar,
    BarChart,
    CartesianGrid,
    ResponsiveContainer,
    Tooltip,
    XAxis,
    YAxis,
} from "recharts";
import type {TooltipPayload} from "recharts";
import type {usagedashboard} from "../../../../../wailsjs/go/models";

const ALL_ITEMS_SELECTION = "all";
const ITEM_SELECTION_PREFIX = "item:";
const STACK_ID = "usage-items";
const STACK_COLORS = [
    "var(--udash-stack-color-1)",
    "var(--udash-stack-color-2)",
    "var(--udash-stack-color-3)",
    "var(--udash-stack-color-4)",
    "var(--udash-stack-color-5)",
    "var(--udash-stack-color-6)",
    "var(--udash-stack-color-7)",
    "var(--udash-stack-color-8)",
] as const;
const MAX_STACKED_SERIES = STACK_COLORS.length;

interface ItemDailyUsageChartProps {
    readonly dailySeries: ReadonlyArray<usagedashboard.DailyUsageSeries>;
    readonly selection: string;
    readonly labelCount: string;
    readonly labelTotal: string;
    readonly labelOther?: string;
    readonly colorVar: `--${string}`;
}

interface SingleChartDatum {
    readonly date: string;
    readonly short: string;
    readonly count: number;
}

interface DailyUsageSeriesLike {
    readonly name: string;
    readonly total_count: number;
    readonly last_used_at: string;
    readonly buckets: ReadonlyArray<DailyUsageBucketLike>;
}

interface DailyUsageBucketLike {
    readonly date: string;
    readonly count: number;
}

export interface StackedTooltipEntry {
    readonly key: string;
    readonly name: string;
    readonly count: number;
    readonly color: string;
}

export type StackedChartDatum = Record<string, string | number | StackedTooltipEntry[]> & {
    date: string;
    short: string;
    total: number;
    entries: StackedTooltipEntry[];
};

export interface StackedSeriesDescriptor {
    readonly key: string;
    readonly name: string;
    readonly color: string;
}

export interface StackedChartData {
    readonly data: StackedChartDatum[];
    readonly series: StackedSeriesDescriptor[];
}

export function ItemDailyUsageChart(props: ItemDailyUsageChartProps) {
    const {dailySeries, selection, labelCount, labelTotal, labelOther = "Other", colorVar} = props;
    const gradientId = `udash-item-bar-grad-${useId().replace(/:/g, "_")}`;
    const barColor = `var(${colorVar})`;
    const selectedSeries = useMemo(
        () => getSelectedSeries(dailySeries, selection),
        [dailySeries, selection],
    );
    const isAllSelection = selection === ALL_ITEMS_SELECTION || !selectedSeries;
    const singleData = useMemo<SingleChartDatum[]>(
        () => isAllSelection || !selectedSeries ? [] : buildSingleItemDailyUsageData(selectedSeries.buckets),
        [isAllSelection, selectedSeries],
    );
    const stackedData = useMemo<StackedChartData>(
        () => isAllSelection ? buildStackedItemDailyUsageData(dailySeries, labelOther) : {data: [], series: []},
        [dailySeries, isAllSelection, labelOther],
    );

    return (
        <div className="usage-dashboard-chart usage-dashboard-item-chart">
            <ResponsiveContainer width="100%" height={160}>
                {isAllSelection ? (
                    <StackedItemsBarChart
                        data={stackedData.data}
                        series={stackedData.series}
                        labelCount={labelCount}
                        labelTotal={labelTotal}
                    />
                ) : (
                    <SingleItemBarChart
                        data={singleData}
                        gradientId={gradientId}
                        barColor={barColor}
                        labelCount={labelCount}
                    />
                )}
            </ResponsiveContainer>
        </div>
    );
}

interface SingleItemBarChartProps {
    readonly data: ReadonlyArray<SingleChartDatum>;
    readonly gradientId: string;
    readonly barColor: string;
    readonly labelCount: string;
}

function SingleItemBarChart({data, gradientId, barColor, labelCount}: SingleItemBarChartProps) {
    return (
        <BarChart data={data} margin={{top: 4, right: 8, left: 0, bottom: 4}}>
            <defs>
                <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={barColor} stopOpacity={0.85}/>
                    <stop offset="100%" stopColor={barColor} stopOpacity={0.35}/>
                </linearGradient>
            </defs>
            <ItemChartAxes/>
            <Tooltip
                cursor={{fill: "var(--udash-chart-cursor)"}}
                contentStyle={{
                    background: "var(--udash-chart-tooltip-bg)",
                    backdropFilter: "blur(8px)",
                    border: "1px solid var(--udash-chart-tooltip-border)",
                    borderRadius: 6,
                    color: "var(--udash-chart-tooltip-fg)",
                    fontSize: 11,
                    boxShadow: "var(--udash-chart-tooltip-shadow)",
                }}
                labelFormatter={(label, payload) => extractTooltipDate(payload) ?? String(label ?? "")}
                formatter={(value) => [String(value ?? 0), labelCount]}
            />
            <Bar dataKey="count" fill={`url(#${gradientId})`} radius={[3, 3, 0, 0]}/>
        </BarChart>
    );
}

interface StackedItemsBarChartProps {
    readonly data: ReadonlyArray<StackedChartDatum>;
    readonly series: ReadonlyArray<StackedSeriesDescriptor>;
    readonly labelCount: string;
    readonly labelTotal: string;
}

function StackedItemsBarChart({data, series, labelCount, labelTotal}: StackedItemsBarChartProps) {
    return (
        <BarChart data={data} margin={{top: 4, right: 8, left: 0, bottom: 4}}>
            <ItemChartAxes/>
            <Tooltip
                cursor={{fill: "var(--udash-chart-cursor)"}}
                wrapperStyle={{pointerEvents: "auto"}}
                content={<StackedItemTooltip labelCount={labelCount} labelTotal={labelTotal}/>}
            />
            {series.map((item) => (
                <Bar
                    key={item.key}
                    dataKey={item.key}
                    name={item.name}
                    stackId={STACK_ID}
                    fill={item.color}
                    radius={[3, 3, 0, 0]}
                />
            ))}
        </BarChart>
    );
}

function ItemChartAxes() {
    return (
        <>
            <CartesianGrid
                strokeDasharray="2 4"
                stroke="var(--udash-chart-grid)"
                vertical={false}
            />
            <XAxis
                dataKey="short"
                tick={{fontSize: 10, fill: "var(--udash-chart-axis)"}}
                tickLine={false}
                axisLine={{stroke: "var(--udash-chart-axis-line)"}}
                interval="preserveStartEnd"
                minTickGap={12}
            />
            <YAxis
                tick={{fontSize: 10, fill: "var(--udash-chart-axis)"}}
                tickLine={false}
                axisLine={false}
                width={26}
                allowDecimals={false}
            />
        </>
    );
}

export function buildSingleItemDailyUsageData(
    buckets: ReadonlyArray<DailyUsageBucketLike>,
): SingleChartDatum[] {
    return buckets.map((bucket) => ({
        date: bucket.date,
        short: bucket.date.slice(5),
        count: normalizedCount(bucket.count),
    }));
}

export function buildStackedItemDailyUsageData(
    dailySeries: ReadonlyArray<DailyUsageSeriesLike>,
    labelOther = "Other",
): StackedChartData {
    const visibleDailySeries = collapseOverflowDailySeries(dailySeries, labelOther);
    const series = visibleDailySeries.map<StackedSeriesDescriptor>((item, index) => ({
        key: `item_${index}`,
        name: item.name,
        color: STACK_COLORS[index] ?? STACK_COLORS[0],
    }));
    const dates = Array.from(new Set(visibleDailySeries.flatMap((item) => item.buckets.map((bucket) => bucket.date))))
        .sort();
    const countsBySeries = visibleDailySeries.map((item) => {
        const countsByDate = new Map<string, number>();
        for (const bucket of item.buckets) {
            countsByDate.set(bucket.date, (countsByDate.get(bucket.date) ?? 0) + normalizedCount(bucket.count));
        }
        return countsByDate;
    });
    const data = dates.map<StackedChartDatum>((date) => {
        const datum: StackedChartDatum = {
            date,
            short: date.slice(5),
            total: 0,
            entries: [],
        };
        for (const [index, item] of series.entries()) {
            const count = countsBySeries[index]?.get(date) ?? 0;
            datum[item.key] = count;
            datum.total += count;
            if (count > 0) {
                datum.entries.push({
                    key: item.key,
                    name: item.name,
                    count,
                    color: item.color,
                });
            }
        }
        datum.entries.sort((left, right) => right.count - left.count || left.name.localeCompare(right.name));
        return datum;
    });
    return {data, series};
}

function collapseOverflowDailySeries(
    dailySeries: ReadonlyArray<DailyUsageSeriesLike>,
    labelOther: string,
): ReadonlyArray<DailyUsageSeriesLike> {
    if (dailySeries.length <= MAX_STACKED_SERIES) {
        return dailySeries;
    }
    const directSeriesLimit = MAX_STACKED_SERIES - 1;
    const directSeries = dailySeries.slice(0, directSeriesLimit);
    const overflowSeries = dailySeries.slice(directSeriesLimit);
    const bucketsByDate = new Map<string, number>();
    let totalCount = 0;
    let lastUsedAt = "";
    for (const series of overflowSeries) {
        totalCount += normalizedCount(series.total_count);
        if (series.last_used_at > lastUsedAt) {
            lastUsedAt = series.last_used_at;
        }
        for (const bucket of series.buckets) {
            bucketsByDate.set(bucket.date, (bucketsByDate.get(bucket.date) ?? 0) + normalizedCount(bucket.count));
        }
    }
    return [
        ...directSeries,
        {
            name: labelOther,
            total_count: totalCount,
            last_used_at: lastUsedAt,
            buckets: Array.from(bucketsByDate.entries())
                .sort(([leftDate], [rightDate]) => leftDate.localeCompare(rightDate))
                .map(([date, count]) => ({date, count})),
        },
    ];
}

export function buildStackedTooltipRows(datum: StackedChartDatum): StackedTooltipEntry[] {
    return datum.entries.filter((entry) => entry.count > 0);
}

function getSelectedSeries(
    dailySeries: ReadonlyArray<usagedashboard.DailyUsageSeries>,
    selection: string,
): usagedashboard.DailyUsageSeries | undefined {
    const selectedName = itemNameFromSelection(selection);
    if (!selectedName) {
        return undefined;
    }
    return dailySeries.find((series) => series.name === selectedName);
}

function itemNameFromSelection(selection: string): string | undefined {
    if (selection === ALL_ITEMS_SELECTION || !selection.startsWith(ITEM_SELECTION_PREFIX)) {
        return undefined;
    }
    try {
        return decodeURIComponent(selection.slice(ITEM_SELECTION_PREFIX.length));
    } catch {
        return undefined;
    }
}

function normalizedCount(count: number): number {
    return Number.isFinite(count) ? Math.max(0, count) : 0;
}

interface StackedItemTooltipOwnProps {
    readonly labelCount: string;
    readonly labelTotal: string;
}

type StackedItemTooltipProps = StackedItemTooltipOwnProps & {
    readonly active?: boolean;
    readonly payload?: ReadonlyArray<{readonly payload?: unknown}>;
};

function StackedItemTooltip(props: StackedItemTooltipProps) {
    const {active, payload, labelCount, labelTotal} = props;
    if (!active) {
        return null;
    }
    const datum = getStackedTooltipDatum(payload);
    if (!datum) {
        return null;
    }
    const rows = buildStackedTooltipRows(datum);
    return (
        <div
            className="usage-dashboard-stacked-tooltip"
            onMouseOver={stopTooltipEventPropagation}
            onMouseMove={stopTooltipEventPropagation}
            onPointerMove={stopTooltipEventPropagation}
            onWheel={stopTooltipEventPropagation}
        >
            <div className="usage-dashboard-stacked-tooltip-date">{datum.date}</div>
            <div className="usage-dashboard-stacked-tooltip-total">
                <span>{labelTotal}</span>
                <strong>{datum.total}</strong>
            </div>
            {rows.length > 0 && (
                <div className="usage-dashboard-stacked-tooltip-list">
                    {rows.map((entry) => (
                        <div key={entry.key} className="usage-dashboard-stacked-tooltip-row">
                            <span
                                className="usage-dashboard-stacked-tooltip-swatch"
                                style={{background: entry.color}}
                            />
                            <span className="usage-dashboard-stacked-tooltip-name">{entry.name}</span>
                            <span className="usage-dashboard-stacked-tooltip-count">
                                {entry.count} {labelCount}
                            </span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}

function stopTooltipEventPropagation(
    event: MouseEvent<HTMLDivElement> | PointerEvent<HTMLDivElement> | WheelEvent<HTMLDivElement>,
) {
    event.stopPropagation();
}

function getStackedTooltipDatum(payload: StackedItemTooltipProps["payload"]): StackedChartDatum | undefined {
    const datum = payload?.[0]?.payload;
    if (!isStackedChartDatum(datum)) {
        return undefined;
    }
    return datum;
}

function isStackedChartDatum(value: unknown): value is StackedChartDatum {
    if (!value || typeof value !== "object") {
        return false;
    }
    const date = Reflect.get(value, "date");
    const total = Reflect.get(value, "total");
    const entries = Reflect.get(value, "entries");
    return typeof date === "string"
        && typeof total === "number"
        && Array.isArray(entries);
}

function extractTooltipDate(payload: TooltipPayload): string | undefined {
    const datum = payload[0]?.payload;
    if (!datum || typeof datum !== "object") {
        return undefined;
    }
    const date = Reflect.get(datum, "date");
    return typeof date === "string" ? date : undefined;
}
