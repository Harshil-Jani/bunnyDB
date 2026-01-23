'use client';

import { useState, useEffect } from 'react';
import { useTheme } from 'next-themes';
import { Sun, Moon, Monitor } from 'lucide-react';

export function ThemeToggle() {
  const [mounted, setMounted] = useState(false);
  const { theme, setTheme } = useTheme();

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    return <div className="w-9 h-9" />;
  }

  const cycleTheme = () => {
    if (theme === 'light') setTheme('dark');
    else if (theme === 'dark') setTheme('system');
    else setTheme('light');
  };

  return (
    <button
      onClick={cycleTheme}
      className="p-2 rounded-lg text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800 transition-colors"
      title={`Theme: ${theme}`}
      aria-label={`Theme: ${theme}`}
    >
      {theme === 'dark' ? (
        <Moon className="w-5 h-5" />
      ) : theme === 'light' ? (
        <Sun className="w-5 h-5" />
      ) : (
        <Monitor className="w-5 h-5" />
      )}
    </button>
  );
}
