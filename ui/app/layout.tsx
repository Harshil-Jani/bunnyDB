import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "BunnyDB",
  description: "PostgreSQL-to-PostgreSQL Replication",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="bg-gray-50 min-h-screen">
        <nav className="bg-white shadow-sm border-b">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex justify-between h-16">
              <div className="flex items-center">
                <span className="text-2xl">🐰</span>
                <span className="ml-2 text-xl font-bold text-gray-900">BunnyDB</span>
              </div>
              <div className="flex items-center space-x-4">
                <a href="/" className="text-gray-600 hover:text-gray-900">Mirrors</a>
                <a href="/peers" className="text-gray-600 hover:text-gray-900">Peers</a>
              </div>
            </div>
          </div>
        </nav>
        <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          {children}
        </main>
      </body>
    </html>
  );
}
