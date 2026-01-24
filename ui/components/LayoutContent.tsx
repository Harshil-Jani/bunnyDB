'use client';

import { useState, useEffect } from 'react';
import { usePathname } from 'next/navigation';
import { LogOut } from 'lucide-react';
import { ThemeProvider } from './ThemeProvider';
import { ThemeToggle } from './ThemeToggle';
import { BunnyLogo } from './BunnyLogo';
import { getToken, getUser, logout, AuthUser } from '../lib/auth';

export function LayoutContent({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isLandingPage = pathname === '/';
  const isLoginPage = pathname === '/login';
  const isPublicPage = isLandingPage || isLoginPage;

  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [user, setUser] = useState<AuthUser | null>(null);

  useEffect(() => {
    if (isPublicPage) {
      setAuthenticated(true);
      return;
    }

    const token = getToken();
    if (!token) {
      window.location.href = '/login';
      return;
    }

    setUser(getUser());
    setAuthenticated(true);
  }, [pathname, isPublicPage]);

  if (authenticated === null) {
    return (
      <ThemeProvider>
        <div className="min-h-screen bg-white dark:bg-gray-950" />
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider>
      {!isPublicPage && (
        <nav className="bg-white dark:bg-gray-900 shadow-sm dark:shadow-gray-900/20 border-b dark:border-gray-800">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex justify-between h-16">
              <div className="flex items-center">
                <a href="/" className="flex items-center">
                  <BunnyLogo size={28} />
                  <span className="ml-2 text-xl font-bold text-gray-900 dark:text-white">BunnyDB</span>
                </a>
              </div>
              <div className="flex items-center space-x-4">
                <a href="/mirrors" className="text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white">Mirrors</a>
                <a href="/peers" className="text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white">Peers</a>
                <a href="/settings" className="text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white">Settings</a>
                <ThemeToggle />
                {user && (
                  <div className="flex items-center gap-3 ml-2 pl-4 border-l border-gray-200 dark:border-gray-700">
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      {user.username}
                      <span className="ml-1 text-[10px] uppercase tracking-wider text-gray-400 dark:text-gray-500">
                        ({user.role})
                      </span>
                    </span>
                    <button
                      onClick={logout}
                      className="p-1.5 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                      title="Sign out"
                    >
                      <LogOut className="w-4 h-4" />
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>
        </nav>
      )}
      {isPublicPage ? (
        <>{children}</>
      ) : (
        <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          {children}
        </main>
      )}
    </ThemeProvider>
  );
}
