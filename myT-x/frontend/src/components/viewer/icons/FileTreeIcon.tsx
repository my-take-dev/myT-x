interface FileTreeIconProps {
  size?: number;
}

export function FileTreeIcon({ size = 20 }: FileTreeIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 20 20"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <rect x="2" y="2" width="7" height="5" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <rect x="8" y="9" width="7" height="4" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <rect x="8" y="15" width="7" height="4" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <path d="M5.5 7V11H8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M5.5 11V17H8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
