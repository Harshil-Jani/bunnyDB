'use client';

import { usePathname } from 'next/navigation';
import { ThemeProvider } from './ThemeProvider';
import { ThemeToggle } from './ThemeToggle';

export function LayoutContent({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isLandingPage = pathname === '/';

  return (
    <ThemeProvider>
      {!isLandingPage && (
        <nav className="bg-white dark:bg-gray-900 shadow-sm dark:shadow-gray-900/20 border-b dark:border-gray-800">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex justify-between h-16">
              <div className="flex items-center">
                <a href="/" className="flex items-center">
                  <span className="text-2xl">🐰</span>
                  <span className="ml-2 text-xl font-bold text-gray-900 dark:text-white">BunnyDB</span>
                </a>
              </div>
              <div className="flex items-center space-x-4">
                <a href="/mirrors" className="text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white">Mirrors</a>
                <a href="/peers" className="text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white">Peers</a>
                <ThemeToggle />
              </div>
            </div>
          </div>
        </nav>
      )}
      {isLandingPage ? (
        <>{children}</>
      ) : (
        <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          {children}
        </main>
      )}
    </ThemeProvider>
  );
}
