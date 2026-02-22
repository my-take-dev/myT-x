import type { CSSProperties } from "react";
import type { GitGraphCommit, LaneAssignment } from "./gitGraphTypes";

// 6-color rotation for lane lines.
const LANE_COLORS = ["#f6d365", "#3de4b7", "#5492ff", "#ff6b6b", "#c084fc", "#f97316"];
const LANE_WIDTH = 20;
const ROW_HEIGHT = 32;
const DOT_RADIUS = 4;

interface CommitRowProps {
  commit: GitGraphCommit;
  laneAssignment: LaneAssignment;
  style: CSSProperties;
  isSelected: boolean;
  onSelect: (commit: GitGraphCommit) => void;
}

function formatRelativeDate(isoDate: string): string {
  const date = new Date(isoDate);
  const now = Date.now();
  const diffMs = now - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return "now";
  if (diffMin < 60) return `${diffMin}m`;
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) return `${diffHours}h`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 30) return `${diffDays}d`;
  const diffMonths = Math.floor(diffDays / 30);
  return `${diffMonths}mo`;
}

export function CommitRow({ commit, laneAssignment, style, isSelected, onSelect }: CommitRowProps) {
  const { lane, connections, activeLaneCount } = laneAssignment;
  const svgWidth = Math.max(activeLaneCount, lane + 1) * LANE_WIDTH + LANE_WIDTH;
  const cx = lane * LANE_WIDTH + LANE_WIDTH / 2;
  const cy = ROW_HEIGHT / 2;
  const isMerge = (commit.parents?.length ?? 0) >= 2;
  const refs = commit.refs ?? [];

  return (
    <div
      className={`commit-row${isSelected ? " selected" : ""}`}
      style={style}
      onClick={() => onSelect(commit)}
    >
      {/* SVG graph segment */}
      <div className="commit-graph-cell" style={{ width: svgWidth }}>
        <svg width={svgWidth} height={ROW_HEIGHT}>
          {/* Active lane vertical lines */}
          {Array.from({ length: Math.max(activeLaneCount, lane + 1) }, (_, i) => (
            <line
              key={`lane-${i}`}
              x1={i * LANE_WIDTH + LANE_WIDTH / 2}
              y1={0}
              x2={i * LANE_WIDTH + LANE_WIDTH / 2}
              y2={ROW_HEIGHT}
              stroke={LANE_COLORS[i % LANE_COLORS.length]}
              strokeWidth={1.5}
              strokeOpacity={0.4}
            />
          ))}

          {/* Connection lines to parents */}
          {connections.map((conn, i) => {
            if (conn.type === "straight") return null;
            const fromX = conn.fromLane * LANE_WIDTH + LANE_WIDTH / 2;
            const toX = conn.toLane * LANE_WIDTH + LANE_WIDTH / 2;
            return (
              <path
                key={`conn-${i}`}
                d={`M ${fromX} ${cy} C ${fromX} ${ROW_HEIGHT}, ${toX} ${cy}, ${toX} ${ROW_HEIGHT}`}
                stroke={LANE_COLORS[conn.toLane % LANE_COLORS.length]}
                strokeWidth={1.5}
                fill="none"
                strokeOpacity={0.7}
              />
            );
          })}

          {/* Commit dot */}
          <circle
            cx={cx}
            cy={cy}
            r={DOT_RADIUS}
            fill={LANE_COLORS[lane % LANE_COLORS.length]}
          />
          {isMerge && (
            <circle
              cx={cx}
              cy={cy}
              r={DOT_RADIUS + 2}
              fill="none"
              stroke={LANE_COLORS[lane % LANE_COLORS.length]}
              strokeWidth={1.5}
            />
          )}
        </svg>
      </div>

      {/* Commit hash */}
      <span className="commit-hash">{commit.hash}</span>

      {/* Ref badges + message */}
      <span className="commit-message">
        {refs.map((ref) => (
          <span key={ref} className={`commit-ref-badge${ref === "HEAD" ? " head" : ""}`}>
            {ref}
          </span>
        ))}
        {commit.subject}
      </span>

      {/* Author */}
      <span className="commit-author">{commit.author_name}</span>

      {/* Relative date */}
      <span className="commit-date">{formatRelativeDate(commit.author_date)}</span>
    </div>
  );
}
