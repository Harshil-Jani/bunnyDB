'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowRight, Database, ArrowDown, MousePointer } from 'lucide-react';
import { BunnyLogo } from '../components/BunnyLogo';
import { ThemeToggle } from '../components/ThemeToggle';

function DemoAnimation() {
  const [batchCount, setBatchCount] = useState(1243);
  const [rowCount, setRowCount] = useState(48721);

  useEffect(() => {
    const interval = setInterval(() => {
      setBatchCount((b) => b + Math.floor(Math.random() * 3) + 1);
      setRowCount((r) => r + Math.floor(Math.random() * 50) + 10);
    }, 2000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="max-w-4xl mx-auto">
      {/* Full mock dashboard */}
      <div className="rounded-xl border border-gray-200 dark:border-gray-800 overflow-hidden shadow-sm">
        {/* Window chrome */}
        <div className="flex items-center gap-1.5 px-4 py-2 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <div className="w-2 h-2 rounded-full bg-red-400" />
          <div className="w-2 h-2 rounded-full bg-yellow-400" />
          <div className="w-2 h-2 rounded-full bg-green-400" />
          <div className="ml-3 flex-1 flex items-center justify-center">
            <div className="px-3 py-0.5 bg-gray-100 dark:bg-gray-800 rounded text-[10px] text-gray-400 font-mono">
              localhost:3000/mirrors
            </div>
          </div>
        </div>

        {/* Mock nav */}
        <div className="flex items-center justify-between px-5 py-2 border-b border-gray-100 dark:border-gray-800 bg-white dark:bg-gray-900">
          <div className="flex items-center gap-2">
            <BunnyLogo size={18} />
            <span className="text-xs font-semibold text-gray-900 dark:text-white">BunnyDB</span>
          </div>
          <div className="flex items-center gap-4 text-[11px] text-gray-500 dark:text-gray-400">
            <span className="text-bunny-600 dark:text-bunny-400 font-medium">Mirrors</span>
            <span>Peers</span>
            <span>Settings</span>
          </div>
        </div>

        {/* Dashboard content */}
        <div className="bg-gray-50 dark:bg-gray-950 p-5">
          {/* Header */}
          <div className="flex items-center justify-between mb-4">
            <span className="text-sm font-semibold text-gray-900 dark:text-white">Mirrors</span>
            <div className="px-2.5 py-1 bg-blue-500 text-white text-[10px] font-medium rounded-md">
              + Create Mirror
            </div>
          </div>

          {/* Live mirror cards */}
          <div className="space-y-3">
            {/* Mirror 1 - Active */}
            <div className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2.5">
                  <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                  <span className="text-xs font-semibold text-gray-900 dark:text-white">prod-to-analytics</span>
                  <span className="text-[9px] font-medium text-green-700 dark:text-green-400 px-1.5 py-0.5 bg-green-100 dark:bg-green-900/30 rounded-full">REPLICATING</span>
                </div>
                <div className="flex items-center gap-1.5">
                  <div className="px-2 py-0.5 text-[9px] text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-800 rounded">Pause</div>
                  <div className="px-2 py-0.5 text-[9px] text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-800 rounded">Resync</div>
                </div>
              </div>
              <div className="grid grid-cols-4 gap-3">
                <div className="text-center">
                  <div className="text-[9px] text-gray-400 mb-0.5">Tables</div>
                  <div className="text-xs font-bold text-gray-900 dark:text-white">5</div>
                </div>
                <div className="text-center">
                  <div className="text-[9px] text-gray-400 mb-0.5">Batches</div>
                  <div className="text-xs font-bold text-gray-900 dark:text-white tabular-nums">{batchCount.toLocaleString()}</div>
                </div>
                <div className="text-center">
                  <div className="text-[9px] text-gray-400 mb-0.5">Rows synced</div>
                  <div className="text-xs font-bold text-gray-900 dark:text-white tabular-nums">{rowCount.toLocaleString()}</div>
                </div>
                <div className="text-center">
                  <div className="text-[9px] text-gray-400 mb-0.5">Lag</div>
                  <div className="text-xs font-bold text-green-600 dark:text-green-400">&lt;1s</div>
                </div>
              </div>
              {/* Animated flow bar */}
              <div className="mt-3 h-1 bg-gray-100 dark:bg-gray-800 rounded-full overflow-hidden">
                <div className="h-full bg-gradient-to-r from-bunny-400 to-green-400 rounded-full animate-demo-flow" />
              </div>
            </div>

            {/* Mirror 2 - Paused */}
            <div className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-4 opacity-70">
              <div className="flex items-center gap-2.5">
                <div className="w-2 h-2 rounded-full bg-yellow-500" />
                <span className="text-xs font-semibold text-gray-900 dark:text-white">staging-sync</span>
                <span className="text-[9px] font-medium text-yellow-700 dark:text-yellow-400 px-1.5 py-0.5 bg-yellow-100 dark:bg-yellow-900/30 rounded-full">PAUSED</span>
              </div>
            </div>
          </div>

          {/* Live log line */}
          <div className="mt-4 px-3 py-2 bg-gray-900 dark:bg-gray-800 rounded-lg font-mono text-[10px] text-gray-300 flex items-center gap-2 overflow-hidden">
            <span className="text-green-400 flex-shrink-0">INF</span>
            <span className="text-gray-500 flex-shrink-0">15:04:32</span>
            <span className="truncate">batch #{batchCount} committed &middot; 23 rows &middot; LSN 0/1A3F{(batchCount % 1000).toString(16).toUpperCase().padStart(3, '0')}0</span>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function LandingPage() {
  const router = useRouter();

  return (
    <div className="min-h-screen bg-white dark:bg-gray-950">
      {/* Nav */}
      <nav className="sticky top-0 z-50 bg-white/80 dark:bg-gray-950/80 backdrop-blur-sm border-b border-gray-100 dark:border-gray-900">
        <div className="max-w-5xl mx-auto px-6 h-14 flex items-center justify-between">
          <a href="/" className="flex items-center gap-2.5">
            <BunnyLogo size={26} />
            <span className="text-[15px] font-semibold text-gray-900 dark:text-white">BunnyDB</span>
          </a>
          <div className="flex items-center gap-6">
            <a href="/mirrors" className="text-[13px] text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white transition-colors">Mirrors</a>
            <a href="/peers" className="text-[13px] text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white transition-colors">Peers</a>
            <ThemeToggle />
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="max-w-5xl mx-auto px-6 pt-24 pb-20">
        <div className="max-w-2xl">
          <h1 className="text-4xl sm:text-5xl font-bold text-gray-900 dark:text-white tracking-tight leading-[1.15]">
            PostgreSQL replication<br />that just works.
          </h1>
          <p className="mt-5 text-lg text-gray-500 dark:text-gray-400 leading-relaxed max-w-lg">
            Real-time Change Data Capture from one Postgres to another.
            Point, click, replicate. No configuration files, no CLI gymnastics.
          </p>
          <div className="mt-8 flex items-center gap-4">
            <button
              onClick={() => router.push('/mirrors')}
              className="inline-flex items-center gap-2 px-5 py-2.5 bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-sm font-medium rounded-lg hover:bg-gray-800 dark:hover:bg-gray-100 transition-colors"
            >
              Open Dashboard
              <ArrowRight className="w-4 h-4" />
            </button>
            <a
              href="#how-it-works"
              className="text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
            >
              How it works
            </a>
          </div>
        </div>
      </section>

      {/* What BunnyDB does — single clear section */}
      <section className="border-t border-gray-100 dark:border-gray-900">
        <div className="max-w-5xl mx-auto px-6 py-20">
          <div className="grid md:grid-cols-3 gap-12">
            <div>
              <div className="text-xs font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wider mb-3">Capture</div>
              <h3 className="text-base font-semibold text-gray-900 dark:text-white mb-2">WAL-based CDC</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                Reads the PostgreSQL write-ahead log via logical replication.
                No triggers, no polling, no application changes needed.
              </p>
            </div>
            <div>
              <div className="text-xs font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wider mb-3">Replicate</div>
              <h3 className="text-base font-semibold text-gray-900 dark:text-white mb-2">Sub-second propagation</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                Changes flow to the destination in real-time. Indexes, schemas,
                and constraints are preserved automatically.
              </p>
            </div>
            <div>
              <div className="text-xs font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wider mb-3">Operate</div>
              <h3 className="text-base font-semibold text-gray-900 dark:text-white mb-2">Pause, resync, recover</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                Full operational control: pause/resume mirrors, resync individual tables
                with zero downtime, and propagate schema changes.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* How it works */}
      <section id="how-it-works" className="border-t border-gray-100 dark:border-gray-900">
        <div className="max-w-5xl mx-auto px-6 py-20">
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-12">How it works</h2>

          {/* Architecture flow — clean, no gradients */}
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4 sm:gap-0 max-w-2xl mx-auto mb-16">
            <div className="flex flex-col items-center gap-2">
              <div className="w-14 h-14 rounded-xl border border-gray-200 dark:border-gray-800 flex items-center justify-center bg-gray-50 dark:bg-gray-900">
                <Database className="w-6 h-6 text-gray-600 dark:text-gray-400" />
              </div>
              <span className="text-xs font-medium text-gray-700 dark:text-gray-300">Source PG</span>
              <span className="text-[11px] text-gray-400">WAL stream</span>
            </div>

            <div className="hidden sm:block w-16 h-px bg-gray-200 dark:bg-gray-800 relative">
              <ArrowRight className="absolute -right-1 -top-1.5 w-3 h-3 text-gray-300 dark:text-gray-600" />
            </div>
            <div className="sm:hidden">
              <ArrowDown className="w-3 h-3 text-gray-300 dark:text-gray-600" />
            </div>

            <div className="flex flex-col items-center gap-2">
              <div className="w-14 h-14 rounded-xl border-2 border-bunny-200 dark:border-bunny-800 flex items-center justify-center bg-bunny-50 dark:bg-bunny-950/30">
                <BunnyLogo size={28} />
              </div>
              <span className="text-xs font-medium text-bunny-600 dark:text-bunny-400">BunnyDB</span>
              <span className="text-[11px] text-gray-400">CDC engine</span>
            </div>

            <div className="hidden sm:block w-16 h-px bg-gray-200 dark:bg-gray-800 relative">
              <ArrowRight className="absolute -right-1 -top-1.5 w-3 h-3 text-gray-300 dark:text-gray-600" />
            </div>
            <div className="sm:hidden">
              <ArrowDown className="w-3 h-3 text-gray-300 dark:text-gray-600" />
            </div>

            <div className="flex flex-col items-center gap-2">
              <div className="w-14 h-14 rounded-xl border border-gray-200 dark:border-gray-800 flex items-center justify-center bg-gray-50 dark:bg-gray-900">
                <Database className="w-6 h-6 text-gray-600 dark:text-gray-400" />
              </div>
              <span className="text-xs font-medium text-gray-700 dark:text-gray-300">Dest PG</span>
              <span className="text-[11px] text-gray-400">Replica</span>
            </div>
          </div>

          {/* Steps */}
          <div className="grid md:grid-cols-3 gap-10">
            <div className="relative pl-8 border-l border-gray-200 dark:border-gray-800">
              <div className="absolute left-0 top-0 -translate-x-1/2 w-5 h-5 rounded-full bg-white dark:bg-gray-950 border-2 border-gray-300 dark:border-gray-700 flex items-center justify-center">
                <span className="text-[10px] font-bold text-gray-500 dark:text-gray-400">1</span>
              </div>
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-1.5">Register peers</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                Add your source and destination PostgreSQL connection details. One-click connectivity test.
              </p>
            </div>
            <div className="relative pl-8 border-l border-gray-200 dark:border-gray-800">
              <div className="absolute left-0 top-0 -translate-x-1/2 w-5 h-5 rounded-full bg-white dark:bg-gray-950 border-2 border-gray-300 dark:border-gray-700 flex items-center justify-center">
                <span className="text-[10px] font-bold text-gray-500 dark:text-gray-400">2</span>
              </div>
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-1.5">Pick tables</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                Browse source schemas, select the tables you want replicated, customize destination mappings if needed.
              </p>
            </div>
            <div className="relative pl-8 border-l border-gray-200 dark:border-gray-800">
              <div className="absolute left-0 top-0 -translate-x-1/2 w-5 h-5 rounded-full bg-white dark:bg-gray-950 border-2 border-bunny-400 dark:border-bunny-500 flex items-center justify-center">
                <span className="text-[10px] font-bold text-bunny-500 dark:text-bunny-400">3</span>
              </div>
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-1.5">Create mirror</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                BunnyDB handles initial snapshot, replication slot setup, and continuous CDC. Monitor everything from the dashboard.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Features — compact list, no cards */}
      <section className="border-t border-gray-100 dark:border-gray-900 bg-gray-50 dark:bg-gray-900/50">
        <div className="max-w-5xl mx-auto px-6 py-20">
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-10">Built for production</h2>

          <div className="grid sm:grid-cols-2 gap-x-16 gap-y-6">
            {[
              { title: 'Temporal-powered workflows', desc: 'Durable execution survives crashes. No data loss, ever.' },
              { title: 'Schema sync', desc: 'Propagate DDL changes to destination without stopping replication.' },
              { title: 'Zero-downtime resync', desc: 'Swap strategy creates shadow tables, copies data, then atomically renames.' },
              { title: 'Per-table control', desc: 'Resync individual tables without touching others. Pause and resume at will.' },
              { title: 'Index preservation', desc: 'Destination indexes are auto-created to match source structure.' },
              { title: 'FK-aware operations', desc: 'Foreign keys are dropped and recreated during swap resync automatically.' },
              { title: 'Event-level logging', desc: 'Searchable, filterable logs categorized by event type — setup, replication, schema, resync.' },
              { title: 'LSN checkpointing', desc: 'Batch IDs and WAL positions tracked for exactly-once delivery guarantees.' },
            ].map((item, i) => (
              <div key={i} className="flex gap-3">
                <div className="mt-1.5 w-1 h-1 rounded-full bg-bunny-500 flex-shrink-0" />
                <div>
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{item.title}</span>
                  <span className="text-sm text-gray-500 dark:text-gray-400 ml-1.5">&mdash; {item.desc}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Live Demo */}
      <section className="border-t border-gray-100 dark:border-gray-900">
        <div className="max-w-5xl mx-auto px-6 py-20">
          <div className="flex items-center justify-between mb-8">
            <div>
              <h2 className="text-2xl font-bold text-gray-900 dark:text-white">See it in action</h2>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">Live counters. Real dashboard feel.</p>
            </div>
            <button
              onClick={() => {
                localStorage.removeItem('bunny_tour_seen');
                router.push('/mirrors');
              }}
              className="inline-flex items-center gap-2 px-4 py-2 text-xs font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:border-gray-300 dark:hover:border-gray-600 transition-colors"
            >
              <MousePointer className="w-3.5 h-3.5" />
              Take Interactive Tour
            </button>
          </div>

          <DemoAnimation />
        </div>
      </section>

      {/* CTA */}
      <section className="border-t border-gray-100 dark:border-gray-900">
        <div className="max-w-5xl mx-auto px-6 py-20 text-center">
          <p className="text-lg text-gray-900 dark:text-white font-medium">
            Your BunnyDB instance is running.
          </p>
          <p className="mt-1.5 text-sm text-gray-500 dark:text-gray-400">
            Start by adding peer connections, then create your first mirror.
          </p>
          <div className="mt-6 flex items-center justify-center gap-4">
            <button
              onClick={() => router.push('/peers')}
              className="inline-flex items-center gap-2 px-5 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:border-gray-300 dark:hover:border-gray-600 transition-colors"
            >
              <Database className="w-4 h-4" />
              Add Peers
            </button>
            <button
              onClick={() => router.push('/mirrors')}
              className="inline-flex items-center gap-2 px-5 py-2.5 bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-sm font-medium rounded-lg hover:bg-gray-800 dark:hover:bg-gray-100 transition-colors"
            >
              Create Mirror
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-100 dark:border-gray-900">
        <div className="max-w-5xl mx-auto px-6 py-6 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <BunnyLogo size={20} />
            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">BunnyDB</span>
          </div>
          <p className="text-xs text-gray-400 dark:text-gray-500">
            PostgreSQL-to-PostgreSQL CDC
          </p>
        </div>
      </footer>
    </div>
  );
}
