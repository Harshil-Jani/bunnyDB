'use client';

import { useRouter } from 'next/navigation';
import { ArrowRight, Zap, Shield, RefreshCw, Database, GitBranch, Table, Activity, Clock, CheckCircle } from 'lucide-react';
import { CapabilitiesGraph } from '../components/CapabilitiesGraph';
import { useState, useEffect } from 'react';

export default function LandingPage() {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  return (
    <div className="landing-page">
      {/* Hero Section */}
      <section className="relative overflow-hidden">
        {/* Animated background grid */}
        <div className="absolute inset-0 opacity-[0.03] dark:opacity-[0.05]">
          <div className="absolute inset-0" style={{
            backgroundImage: `radial-gradient(circle at 1px 1px, currentColor 1px, transparent 0)`,
            backgroundSize: '40px 40px',
          }} />
        </div>

        {/* Gradient orbs */}
        <div className="absolute top-20 -left-32 w-96 h-96 bg-bunny-500/10 dark:bg-bunny-500/5 rounded-full blur-3xl" />
        <div className="absolute bottom-0 -right-32 w-96 h-96 bg-blue-500/10 dark:bg-blue-500/5 rounded-full blur-3xl" />

        <div className="relative max-w-6xl mx-auto px-6 pt-20 pb-24 sm:pt-28 sm:pb-32">
          {/* Badge */}
          <div className={`inline-flex items-center gap-2 px-4 py-2 rounded-full bg-bunny-50 dark:bg-bunny-950/50 border border-bunny-200 dark:border-bunny-800 mb-8 transition-all duration-700 ${mounted ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-4'}`}>
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-bunny-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-2 w-2 bg-bunny-500"></span>
            </span>
            <span className="text-sm font-medium text-bunny-700 dark:text-bunny-300">PostgreSQL CDC Replication</span>
          </div>

          {/* Headline */}
          <h1 className={`transition-all duration-700 delay-100 ${mounted ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-4'}`}>
            <span className="block text-5xl sm:text-6xl lg:text-7xl font-black text-gray-900 dark:text-white tracking-tight leading-[1.1]">
              Replicate Postgres.
            </span>
            <span className="block text-5xl sm:text-6xl lg:text-7xl font-black tracking-tight leading-[1.1] mt-2">
              <span className="text-transparent bg-clip-text bg-gradient-to-r from-bunny-500 via-bunny-600 to-orange-600 dark:from-bunny-400 dark:via-bunny-500 dark:to-orange-500">
                Fast as a bunny.
              </span>
            </span>
          </h1>

          {/* Subtitle */}
          <p className={`mt-8 text-xl sm:text-2xl text-gray-600 dark:text-gray-300 max-w-2xl leading-relaxed transition-all duration-700 delay-200 ${mounted ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-4'}`}>
            Real-time Change Data Capture from one PostgreSQL to another.
            Schema-aware, index-preserving, zero-downtime replication
            with a UI you&apos;ll actually enjoy using.
          </p>

          {/* CTA Buttons */}
          <div className={`mt-10 flex flex-wrap gap-4 transition-all duration-700 delay-300 ${mounted ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-4'}`}>
            <button
              onClick={() => router.push('/mirrors')}
              className="group inline-flex items-center gap-3 px-8 py-4 bg-bunny-500 hover:bg-bunny-600 text-white font-semibold rounded-xl shadow-lg shadow-bunny-500/25 hover:shadow-bunny-500/40 transition-all duration-200 hover:-translate-y-0.5"
            >
              Open Dashboard
              <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
            </button>
            <button
              onClick={() => router.push('/peers')}
              className="inline-flex items-center gap-3 px-8 py-4 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 font-semibold rounded-xl border border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 shadow-sm hover:shadow-md transition-all duration-200 hover:-translate-y-0.5"
            >
              <Database className="w-5 h-5" />
              Configure Peers
            </button>
          </div>

          {/* Stats row */}
          <div className={`mt-16 grid grid-cols-2 sm:grid-cols-4 gap-8 transition-all duration-700 delay-500 ${mounted ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'}`}>
            {[
              { value: 'CDC', label: 'Logical Replication' },
              { value: '< 1s', label: 'Propagation Delay' },
              { value: '∞', label: 'Tables Supported' },
              { value: '24/7', label: 'Continuous Sync' },
            ].map((stat, i) => (
              <div key={i} className="text-center sm:text-left">
                <div className="text-2xl sm:text-3xl font-black text-gray-900 dark:text-white">{stat.value}</div>
                <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">{stat.label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Problem / Solution Section */}
      <section className="relative bg-white dark:bg-gray-900 border-y border-gray-200 dark:border-gray-800">
        <div className="max-w-6xl mx-auto px-6 py-20 sm:py-28">
          <div className="grid lg:grid-cols-2 gap-16 items-center">
            {/* Problem */}
            <div>
              <span className="text-sm font-bold uppercase tracking-widest text-red-500 dark:text-red-400">The Problem</span>
              <h2 className="mt-4 text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white leading-tight">
                PostgreSQL replication is harder than it should be
              </h2>
              <div className="mt-6 space-y-4">
                {[
                  'Setting up logical replication requires deep PostgreSQL expertise',
                  'Managing replication slots, publications, and subscriptions manually is error-prone',
                  'Schema changes break replication with no easy recovery',
                  'No visibility into sync status, lag, or per-table progress',
                  'Existing tools are over-engineered for simple Postgres-to-Postgres use cases',
                ].map((problem, i) => (
                  <div key={i} className="flex items-start gap-3">
                    <div className="mt-1.5 w-1.5 h-1.5 rounded-full bg-red-400 dark:bg-red-500 flex-shrink-0" />
                    <p className="text-gray-600 dark:text-gray-300">{problem}</p>
                  </div>
                ))}
              </div>
            </div>

            {/* Solution */}
            <div className="lg:pl-8">
              <span className="text-sm font-bold uppercase tracking-widest text-bunny-500 dark:text-bunny-400">The Solution</span>
              <h2 className="mt-4 text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white leading-tight">
                BunnyDB makes it effortless
              </h2>
              <div className="mt-6 space-y-4">
                {[
                  'Point-and-click mirror setup — select tables, hit create',
                  'Automatic slot, publication, and subscription management',
                  'Built-in schema sync: propagate DDL changes without downtime',
                  'Real-time dashboard with per-table status, row counts, and logs',
                  'Purpose-built for Postgres → Postgres with nothing you don\'t need',
                ].map((solution, i) => (
                  <div key={i} className="flex items-start gap-3">
                    <CheckCircle className="w-5 h-5 text-bunny-500 mt-0.5 flex-shrink-0" />
                    <p className="text-gray-600 dark:text-gray-300">{solution}</p>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Interactive Capabilities Graph */}
      <section className="max-w-6xl mx-auto px-6 py-20 sm:py-28">
        <div className="text-center mb-12">
          <span className="text-sm font-bold uppercase tracking-widest text-bunny-500 dark:text-bunny-400">Capabilities</span>
          <h2 className="mt-4 text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white">
            Everything you need for reliable replication
          </h2>
          <p className="mt-3 text-gray-500 dark:text-gray-400 text-sm">
            Hover or click on nodes to explore features
          </p>
        </div>

        <CapabilitiesGraph />
      </section>

      {/* Architecture / How it Works */}
      <section className="bg-gray-900 dark:bg-gray-950 text-white border-y border-gray-800">
        <div className="max-w-6xl mx-auto px-6 py-20 sm:py-28">
          <div className="text-center mb-16">
            <span className="text-sm font-bold uppercase tracking-widest text-bunny-400">Architecture</span>
            <h2 className="mt-4 text-3xl sm:text-4xl font-bold">How BunnyDB works</h2>
          </div>

          {/* Flow diagram */}
          <div className="grid sm:grid-cols-5 gap-4 items-center max-w-4xl mx-auto">
            {/* Source DB */}
            <div className="text-center p-4">
              <div className="mx-auto w-16 h-16 rounded-2xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center mb-3">
                <Database className="w-8 h-8 text-blue-400" />
              </div>
              <div className="text-sm font-semibold">Source PG</div>
              <div className="text-xs text-gray-400 mt-1">WAL Producer</div>
            </div>

            {/* Arrow */}
            <div className="hidden sm:flex items-center justify-center">
              <div className="w-full h-px bg-gradient-to-r from-blue-500/50 to-bunny-500/50 relative">
                <ArrowRight className="absolute -right-2 -top-2 w-4 h-4 text-bunny-400" />
              </div>
            </div>

            {/* BunnyDB Engine */}
            <div className="text-center p-4">
              <div className="mx-auto w-20 h-20 rounded-2xl bg-bunny-500/10 border-2 border-bunny-500/30 flex items-center justify-center mb-3 relative">
                <span className="text-3xl">🐰</span>
                <div className="absolute -top-1 -right-1 w-3 h-3 bg-green-400 rounded-full border-2 border-gray-900 dark:border-gray-950"></div>
              </div>
              <div className="text-sm font-semibold text-bunny-300">BunnyDB</div>
              <div className="text-xs text-gray-400 mt-1">CDC Engine</div>
            </div>

            {/* Arrow */}
            <div className="hidden sm:flex items-center justify-center">
              <div className="w-full h-px bg-gradient-to-r from-bunny-500/50 to-green-500/50 relative">
                <ArrowRight className="absolute -right-2 -top-2 w-4 h-4 text-green-400" />
              </div>
            </div>

            {/* Destination DB */}
            <div className="text-center p-4">
              <div className="mx-auto w-16 h-16 rounded-2xl bg-green-500/10 border border-green-500/20 flex items-center justify-center mb-3">
                <Database className="w-8 h-8 text-green-400" />
              </div>
              <div className="text-sm font-semibold">Dest PG</div>
              <div className="text-xs text-gray-400 mt-1">Replica</div>
            </div>
          </div>

          {/* Architecture details */}
          <div className="mt-16 grid sm:grid-cols-3 gap-8 text-center">
            {[
              {
                icon: <Clock className="w-5 h-5" />,
                title: 'Temporal Workflows',
                desc: 'Durable execution ensures no data loss even through crashes and restarts',
              },
              {
                icon: <GitBranch className="w-5 h-5" />,
                title: 'Logical Decoding',
                desc: 'pglogrepl reads the WAL stream directly — no triggers, no polling, no overhead',
              },
              {
                icon: <Shield className="w-5 h-5" />,
                title: 'Exactly-once Delivery',
                desc: 'LSN tracking and batch IDs guarantee consistency across source and destination',
              },
            ].map((item, i) => (
              <div key={i} className="p-6 rounded-xl bg-gray-800/50 border border-gray-700/50">
                <div className="inline-flex p-2 rounded-lg bg-gray-700/50 text-gray-300 mb-3">
                  {item.icon}
                </div>
                <h3 className="font-semibold text-white">{item.title}</h3>
                <p className="mt-2 text-sm text-gray-400 leading-relaxed">{item.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Quick Start */}
      <section className="max-w-6xl mx-auto px-6 py-20 sm:py-28">
        <div className="text-center mb-16">
          <span className="text-sm font-bold uppercase tracking-widest text-bunny-500 dark:text-bunny-400">Get Started</span>
          <h2 className="mt-4 text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white">
            Up and running in 3 steps
          </h2>
        </div>

        <div className="grid sm:grid-cols-3 gap-8">
          {[
            {
              step: '01',
              title: 'Add your databases',
              description: 'Register source and destination PostgreSQL connections as peers. Test connectivity with one click.',
            },
            {
              step: '02',
              title: 'Select tables',
              description: 'Browse schemas, pick tables to replicate, and optionally customize destination mappings.',
            },
            {
              step: '03',
              title: 'Create & monitor',
              description: 'Hit create — BunnyDB handles snapshot, CDC setup, and ongoing sync. Monitor everything in real-time.',
            },
          ].map((item, i) => (
            <div key={i} className="relative">
              <div className="text-7xl font-black text-gray-100 dark:text-gray-800 absolute -top-4 -left-2 select-none">{item.step}</div>
              <div className="relative pt-12 pl-2">
                <h3 className="text-xl font-bold text-gray-900 dark:text-white">{item.title}</h3>
                <p className="mt-3 text-gray-600 dark:text-gray-400 leading-relaxed">{item.description}</p>
              </div>
            </div>
          ))}
        </div>

        {/* Final CTA */}
        <div className="mt-20 text-center">
          <div className="inline-flex flex-col items-center p-10 rounded-3xl bg-gradient-to-br from-bunny-50 to-orange-50 dark:from-bunny-950/30 dark:to-orange-950/20 border border-bunny-100 dark:border-bunny-900/30">
            <span className="text-5xl mb-4">🐰</span>
            <h3 className="text-2xl font-bold text-gray-900 dark:text-white">Ready to replicate?</h3>
            <p className="mt-2 text-gray-600 dark:text-gray-400 max-w-md">
              Your BunnyDB instance is running. Start by adding peer connections to your PostgreSQL databases.
            </p>
            <button
              onClick={() => router.push('/mirrors')}
              className="mt-6 group inline-flex items-center gap-3 px-8 py-4 bg-bunny-500 hover:bg-bunny-600 text-white font-semibold rounded-xl shadow-lg shadow-bunny-500/25 hover:shadow-bunny-500/40 transition-all duration-200 hover:-translate-y-0.5"
            >
              Go to Dashboard
              <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
            </button>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
        <div className="max-w-6xl mx-auto px-6 py-8 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-xl">🐰</span>
            <span className="font-bold text-gray-900 dark:text-white">BunnyDB</span>
          </div>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            PostgreSQL-to-PostgreSQL Replication Engine
          </p>
        </div>
      </footer>
    </div>
  );
}
