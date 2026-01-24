'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowRight, Database, ArrowDown, MousePointer } from 'lucide-react';
import { BunnyLogo } from '../components/BunnyLogo';
import { ThemeToggle } from '../components/ThemeToggle';

const demoSteps = [
  {
    label: 'Register Peers',
    description: 'Add source & destination PostgreSQL connections',
    visual: (
      <div className="space-y-2">
        <div className="flex items-center gap-2 px-3 py-2 bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700">
          <Database className="w-4 h-4 text-blue-500" />
          <span className="text-xs font-medium text-gray-700 dark:text-gray-200">production-db</span>
          <span className="ml-auto text-[10px] text-green-600 dark:text-green-400 font-medium">Connected</span>
        </div>
        <div className="flex items-center gap-2 px-3 py-2 bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700">
          <Database className="w-4 h-4 text-purple-500" />
          <span className="text-xs font-medium text-gray-700 dark:text-gray-200">analytics-replica</span>
          <span className="ml-auto text-[10px] text-green-600 dark:text-green-400 font-medium">Connected</span>
        </div>
      </div>
    ),
  },
  {
    label: 'Select Tables',
    description: 'Choose which tables to replicate',
    visual: (
      <div className="space-y-1.5">
        {['users', 'orders', 'products', 'payments'].map((t, i) => (
          <div key={t} className="flex items-center gap-2 px-3 py-1.5 bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700">
            <div className={`w-3.5 h-3.5 rounded border-2 flex items-center justify-center ${i < 3 ? 'border-blue-500 bg-blue-500' : 'border-gray-300 dark:border-gray-600'}`}>
              {i < 3 && <svg className="w-2 h-2 text-white" fill="currentColor" viewBox="0 0 12 12"><path d="M10.28 2.28L3.989 8.575 1.695 6.28A1 1 0 00.28 7.695l3 3a1 1 0 001.414 0l7-7A1 1 0 0010.28 2.28z"/></svg>}
            </div>
            <span className="text-xs text-gray-700 dark:text-gray-200 font-mono">public.{t}</span>
          </div>
        ))}
      </div>
    ),
  },
  {
    label: 'Monitor Replication',
    description: 'Real-time status, logs, and controls',
    visual: (
      <div className="space-y-2">
        <div className="flex items-center justify-between px-3 py-2 bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
            <span className="text-xs font-medium text-gray-700 dark:text-gray-200">prod-to-analytics</span>
          </div>
          <span className="text-[10px] font-medium text-green-600 dark:text-green-400 px-2 py-0.5 bg-green-50 dark:bg-green-900/30 rounded-full">REPLICATING</span>
        </div>
        <div className="grid grid-cols-3 gap-2">
          <div className="px-2 py-1.5 bg-gray-50 dark:bg-gray-800/50 rounded text-center">
            <div className="text-[10px] text-gray-400">Tables</div>
            <div className="text-xs font-bold text-gray-700 dark:text-gray-200">3</div>
          </div>
          <div className="px-2 py-1.5 bg-gray-50 dark:bg-gray-800/50 rounded text-center">
            <div className="text-[10px] text-gray-400">Batch</div>
            <div className="text-xs font-bold text-gray-700 dark:text-gray-200">1,247</div>
          </div>
          <div className="px-2 py-1.5 bg-gray-50 dark:bg-gray-800/50 rounded text-center">
            <div className="text-[10px] text-gray-400">Lag</div>
            <div className="text-xs font-bold text-gray-700 dark:text-gray-200">&lt;1s</div>
          </div>
        </div>
      </div>
    ),
  },
];

function DemoAnimation() {
  const [activeStep, setActiveStep] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setActiveStep((s) => (s + 1) % demoSteps.length);
    }, 3500);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="max-w-3xl mx-auto">
      {/* Step tabs */}
      <div className="flex items-center justify-center gap-1 mb-6">
        {demoSteps.map((step, i) => (
          <button
            key={i}
            onClick={() => setActiveStep(i)}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium transition-all ${
              i === activeStep
                ? 'bg-gray-900 dark:bg-white text-white dark:text-gray-900 shadow-sm'
                : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800'
            }`}
          >
            <span className="w-4 h-4 rounded-full border-2 flex items-center justify-center text-[9px] font-bold border-current">
              {i + 1}
            </span>
            {step.label}
          </button>
        ))}
      </div>

      {/* Demo window */}
      <div className="bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-800 overflow-hidden">
        {/* Window chrome */}
        <div className="flex items-center gap-1.5 px-4 py-2.5 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <div className="w-2.5 h-2.5 rounded-full bg-red-400" />
          <div className="w-2.5 h-2.5 rounded-full bg-yellow-400" />
          <div className="w-2.5 h-2.5 rounded-full bg-green-400" />
          <div className="ml-3 flex-1 flex items-center justify-center">
            <div className="px-3 py-0.5 bg-gray-100 dark:bg-gray-800 rounded text-[10px] text-gray-400 font-mono">
              localhost:3000/{activeStep === 0 ? 'peers' : activeStep === 1 ? 'mirrors/new' : 'mirrors'}
            </div>
          </div>
        </div>

        {/* Content */}
        <div className="p-6 min-h-[200px] flex flex-col justify-center">
          <div className="max-w-xs mx-auto w-full">
            <p className="text-xs text-gray-400 dark:text-gray-500 uppercase tracking-wider font-medium mb-1">
              Step {activeStep + 1}
            </p>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-1">
              {demoSteps[activeStep].label}
            </h3>
            <p className="text-xs text-gray-500 dark:text-gray-400 mb-4">
              {demoSteps[activeStep].description}
            </p>
            {demoSteps[activeStep].visual}
          </div>
        </div>

        {/* Progress dots */}
        <div className="flex items-center justify-center gap-1.5 pb-4">
          {demoSteps.map((_, i) => (
            <div
              key={i}
              className={`h-1 rounded-full transition-all duration-300 ${
                i === activeStep ? 'w-6 bg-bunny-500' : 'w-1.5 bg-gray-300 dark:bg-gray-700'
              }`}
            />
          ))}
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

      {/* Interactive Demo Walkthrough */}
      <section className="border-t border-gray-100 dark:border-gray-900">
        <div className="max-w-5xl mx-auto px-6 py-20">
          <div className="text-center mb-12">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-3">See it in action</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 max-w-md mx-auto">
              A quick walkthrough of the BunnyDB dashboard — from adding peers to monitoring active mirrors.
            </p>
          </div>

          {/* Animated UI Mockup */}
          <DemoAnimation />

          <div className="mt-10 text-center">
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
              Want a hands-on tour? Log in and we'll guide you through the interface step by step.
            </p>
            <button
              onClick={() => {
                localStorage.removeItem('bunny_tour_seen');
                router.push('/mirrors');
              }}
              className="inline-flex items-center gap-2 px-5 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:border-gray-300 dark:hover:border-gray-600 transition-colors"
            >
              <MousePointer className="w-4 h-4" />
              Take the Interactive Tour
            </button>
          </div>
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
