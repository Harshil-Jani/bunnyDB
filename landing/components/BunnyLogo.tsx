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
      className={`bunny-logo ${className}`}
    >
      <style>{`
        .bunny-logo .ear-left { transition: transform 0.3s ease; transform-origin: 23px 26px; }
        .bunny-logo .ear-right { transition: transform 0.3s ease; transform-origin: 41px 24px; }
        .bunny-logo:hover .ear-left { transform: rotate(-8deg); }
        .bunny-logo:hover .ear-right { transform: rotate(8deg); }
        .bunny-logo .wink-group { transition: transform 0.2s ease; transform-origin: 38px 34px; }
        .bunny-logo:hover .wink-group { transform: scaleY(0.1); }
        .bunny-logo .wink-line { transition: opacity 0.2s ease; opacity: 0; }
        .bunny-logo:hover .wink-line { opacity: 1; }
        .bunny-logo .tongue { transition: transform 0.25s ease; transform-origin: 32px 47px; transform: scaleY(0); }
        .bunny-logo:hover .tongue { transform: scaleY(1); }
      `}</style>

      {/* Background */}
      <circle cx="32" cy="32" r="30" fill="#4a90d9" />
      {/* Left ear - wiggles on hover */}
      <g className="ear-left">
        <ellipse cx="23" cy="12" rx="5.5" ry="14" fill="#e8e8e8" transform="rotate(-12 23 12)" />
        <ellipse cx="23" cy="12" rx="3" ry="10" fill="#ffb8b8" transform="rotate(-12 23 12)" />
      </g>
      {/* Right ear - wiggles on hover */}
      <g className="ear-right">
        <ellipse cx="41" cy="10" rx="5.5" ry="14" fill="#e8e8e8" transform="rotate(6 41 10)" />
        <ellipse cx="41" cy="10" rx="3" ry="10" fill="#ffb8b8" transform="rotate(6 41 10)" />
      </g>
      {/* Face */}
      <ellipse cx="32" cy="38" rx="17" ry="18" fill="#e8e8e8" />
      {/* Cheek/muzzle area */}
      <ellipse cx="32" cy="42" rx="10" ry="9" fill="white" />
      {/* Left eye - stays open */}
      <ellipse cx="26" cy="34" rx="3.5" ry="4" fill="white" />
      <circle cx="27" cy="35" r="2.2" fill="#411506" />
      <circle cx="27.8" cy="34" r="0.9" fill="white" />
      {/* Right eye - winks on hover (group scales to flat) */}
      <g className="wink-group">
        <ellipse cx="38" cy="34" rx="3.5" ry="4" fill="white" />
        <circle cx="39" cy="35" r="2.2" fill="#411506" />
        <circle cx="39.8" cy="34" r="0.9" fill="white" />
      </g>
      {/* Wink line - appears on hover */}
      <path className="wink-line" d="M35 34 Q38 32.5 41 34" stroke="#411506" strokeWidth="1.5" fill="none" strokeLinecap="round" />
      {/* Eyebrows */}
      <path d="M23 29.5 Q26 28 29.5 29" stroke="#411506" strokeWidth="1.2" fill="none" strokeLinecap="round" />
      <path d="M35 29 Q38 28 41 29.5" stroke="#411506" strokeWidth="1.2" fill="none" strokeLinecap="round" />
      {/* Nose */}
      <ellipse cx="32" cy="40" rx="2.5" ry="2" fill="#f39333" />
      {/* Buck teeth */}
      <rect x="29.5" y="44" width="2.5" height="3.5" rx="0.8" fill="white" stroke="#ccc" strokeWidth="0.4" />
      <rect x="32" y="44" width="2.5" height="3.5" rx="0.8" fill="white" stroke="#ccc" strokeWidth="0.4" />
      {/* Tongue - peeks out on hover */}
      <ellipse className="tongue" cx="32" cy="49" rx="2.5" ry="2.5" fill="#e85d5d" />
      {/* Smirk */}
      <path d="M27 44 Q32 46.5 37 44" stroke="#943610" strokeWidth="1.2" fill="none" strokeLinecap="round" />
      {/* Whiskers */}
      <line x1="14" y1="38" x2="22" y2="40" stroke="#aaa" strokeWidth="0.7" strokeLinecap="round" />
      <line x1="14" y1="42" x2="22" y2="42" stroke="#aaa" strokeWidth="0.7" strokeLinecap="round" />
      <line x1="42" y1="40" x2="50" y2="38" stroke="#aaa" strokeWidth="0.7" strokeLinecap="round" />
      <line x1="42" y1="42" x2="50" y2="42" stroke="#aaa" strokeWidth="0.7" strokeLinecap="round" />
    </svg>
  );
}
