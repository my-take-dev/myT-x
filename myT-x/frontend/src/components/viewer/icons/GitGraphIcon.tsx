interface GitGraphIconProps {
  size?: number;
}

export function GitGraphIcon({ size = 20 }: GitGraphIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 20 20"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <circle cx="6" cy="4" r="2" stroke="currentColor" strokeWidth="1.5" />
      <circle cx="6" cy="16" r="2" stroke="currentColor" strokeWidth="1.5" />
      <circle cx="14" cy="10" r="2" stroke="currentColor" strokeWidth="1.5" />
      <path d="M6 6V14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M7.5 5.5C10 7 12.5 8.5 12.5 10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}
