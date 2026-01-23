interface BunnyLogoProps {
  size?: number;
  className?: string;
}

export function BunnyLogo({ size = 32, className = '' }: BunnyLogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 64 64"
      fill="none"
      width={size}
      height={size}
      className={className}
    >
      <circle cx="32" cy="32" r="30" fill="#f0750f" />
      <ellipse cx="24" cy="14" rx="5" ry="12" fill="white" transform="rotate(-8 24 14)" />
      <ellipse cx="24" cy="14" rx="3" ry="9" fill="#fad7a5" transform="rotate(-8 24 14)" />
      <ellipse cx="40" cy="14" rx="5" ry="12" fill="white" transform="rotate(8 40 14)" />
      <ellipse cx="40" cy="14" rx="3" ry="9" fill="#fad7a5" transform="rotate(8 40 14)" />
      <circle cx="32" cy="36" r="18" fill="white" />
      <circle cx="26" cy="33" r="3" fill="#411506" />
      <circle cx="38" cy="33" r="3" fill="#411506" />
      <circle cx="27.5" cy="31.5" r="1.2" fill="white" />
      <circle cx="39.5" cy="31.5" r="1.2" fill="white" />
      <ellipse cx="32" cy="39" rx="2.5" ry="2" fill="#f39333" />
      <path d="M29 42 Q32 45 35 42" stroke="#943610" strokeWidth="1.5" fill="none" strokeLinecap="round" />
      <circle cx="22" cy="39" r="3" fill="#fad7a5" opacity="0.6" />
      <circle cx="42" cy="39" r="3" fill="#fad7a5" opacity="0.6" />
    </svg>
  );
}
