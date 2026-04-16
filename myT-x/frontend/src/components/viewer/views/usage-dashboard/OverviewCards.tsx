interface OverviewCardsProps {
    readonly items: ReadonlyArray<{id: string; label: string; value: string; sub?: string}>;
}

export function OverviewCards({items}: OverviewCardsProps) {
    return (
        <div className="usage-dashboard-overview">
            {items.map((item) => (
                <div key={item.id} className="usage-dashboard-card">
                    <div className="usage-dashboard-card-label">{item.label}</div>
                    <div className="usage-dashboard-card-value" title={item.value}>{item.value}</div>
                    {item.sub ? <div className="usage-dashboard-card-sub">{item.sub}</div> : null}
                </div>
            ))}
        </div>
    );
}
