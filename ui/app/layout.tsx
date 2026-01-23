import type { Metadata } from "next";
import "./globals.css";
import { LayoutContent } from "../components/LayoutContent";

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
    <html lang="en" suppressHydrationWarning>
      <body className="bg-gray-50 dark:bg-gray-950 min-h-screen">
        <LayoutContent>{children}</LayoutContent>
      </body>
    </html>
  );
}
