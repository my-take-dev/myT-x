import {useId, useMemo, useState} from "react";
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

type SeriesKey = "sessions" | "secondary" | "tool_calls";
type ChartDatum = {
    date: string;
    short: string;
    sessions: number;
    secondary: number;
    tool_calls: number;
};

interface DailyActivityChartProps {
    readonly buckets: ReadonlyArray<usagedashboard.DailyBucket>;
    readonly labelSessions: string;
    readonly labelSecondary: string;
    readonly labelToolCalls: string;
    readonly colorVar: `--${string}`;
}

export function DailyActivityChart(props: DailyActivityChartProps) {
    const {buckets, labelSessions, labelSecondary, labelToolCalls, colorVar} = props;
    const [series, setSeries] = useState<SeriesKey>("sessions");
    // Instance-scoped IDs avoid SVG <defs> collisions even when multiple
    // charts render with the same accent color.
    const gradientId = `udash-bar-grad-${useId().replace(/:/g, "_")}`;
    const barColor = `var(${colorVar})`;

    const data = useMemo<ChartDatum[]>(
        () =>
            buckets.map((b) => ({
                date: b.date,
                short: b.date.slice(5), // "MM-DD"
                sessions: b.sessions,
                secondary: b.secondary,
                tool_calls: b.tool_calls,
            })),
        [buckets],
    );

    const labelMap: Record<SeriesKey, string> = {
        sessions: labelSessions,
        secondary: labelSecondary,
        tool_calls: labelToolCalls,
    };

    return (
        <div className="usage-dashboard-chart">
            <div className="usage-dashboard-chart-series" role="tablist">
                {(Object.keys(labelMap) as SeriesKey[]).map((key) => (
                    <button
                        key={key}
                        type="button"
                        role="tab"
                        aria-selected={series === key}
                        className="usage-dashboard-chart-series-btn"
                        onClick={() => setSeries(key)}
                    >
                        {labelMap[key]}
                    </button>
                ))}
            </div>
            <ResponsiveContainer width="100%" height={160}>
                <BarChart data={data} margin={{top: 4, right: 8, left: 0, bottom: 4}}>
                    {/* SVG defs for vertical gradient fill — Recharts 3.x compatible */}
                    <defs>
                        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                            <stop offset="0%" stopColor={barColor} stopOpacity={0.85}/>
                            <stop offset="100%" stopColor={barColor} stopOpacity={0.35}/>
                        </linearGradient>
                    </defs>
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
                        formatter={(value) => [String(value ?? 0), labelMap[series]]}
                    />
                    <Bar dataKey={series} fill={`url(#${gradientId})`} radius={[3, 3, 0, 0]}/>
                </BarChart>
            </ResponsiveContainer>
        </div>
    );
}

function extractTooltipDate(payload: TooltipPayload): string | undefined {
    const datum = payload[0]?.payload;
    if (!datum || typeof datum !== "object") {
        return undefined;
    }
    const date = Reflect.get(datum, "date");
    return typeof date === "string" ? date : undefined;
}
