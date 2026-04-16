import {useMemo, useState} from "react";
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
    readonly color: string;
}

export function DailyActivityChart(props: DailyActivityChartProps) {
    const {buckets, labelSessions, labelSecondary, labelToolCalls, color} = props;
    const [series, setSeries] = useState<SeriesKey>("sessions");

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
                    <CartesianGrid stroke="#2d2d2d" vertical={false}/>
                    <XAxis
                        dataKey="short"
                        tick={{fontSize: 10, fill: "#858585"}}
                        interval="preserveStartEnd"
                        minTickGap={12}
                    />
                    <YAxis
                        tick={{fontSize: 10, fill: "#858585"}}
                        width={26}
                        allowDecimals={false}
                    />
                    <Tooltip
                        cursor={{fill: "rgba(255,255,255,0.04)"}}
                        contentStyle={{
                            background: "#1e1e1e",
                            border: "1px solid #2d2d2d",
                            color: "#d4d4d4",
                            fontSize: 11,
                        }}
                        labelFormatter={(label, payload) => extractTooltipDate(payload) ?? String(label ?? "")}
                        formatter={(value) => [String(value ?? 0), labelMap[series]]}
                    />
                    <Bar dataKey={series} fill={color} radius={[2, 2, 0, 0]}/>
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
