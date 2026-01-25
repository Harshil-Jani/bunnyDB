'use client';

import { useState, useEffect } from 'react';
import { ArrowRight, Database, ArrowDown, CheckCircle, Play, Pause, RefreshCw, FileText, Github, BookOpen } from 'lucide-react';
import { BunnyLogo } from '../components/BunnyLogo';
import { ThemeToggle } from '../components/ThemeToggle';

const basePath = '/bunnyDB';

const screens = [
  { id: 'peers', label: 'Add Peers', route: 'peers' },
  { id: 'create', label: 'Create Mirror', route: 'mirrors/new' },
  { id: 'tables', label: 'Select Tables', route: 'mirrors/new' },
  { id: 'monitor', label: 'Monitor', route: 'mirrors' },
  { id: 'controls', label: 'Controls', route: 'mirrors/prod-to-analytics' },
];

function PeersScreen() {
  const [tested, setTested] = useState([false, false]);
  useEffect(() => {
    const t1 = setTimeout(() => setTested([true, false]), 1200);
    const t2 = setTimeout(() => setTested([true, true]), 2200);
    return () => { clearTimeout(t1); clearTimeout(t2); };
  }, []);

  return (
    <div className="space-y-2.5">
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs font-semibold text-gray-900 dark:text-white">Peers</span>
        <div className="px-2 py-0.5 bg-blue-500 text-white text-[9px] font-medium rounded">+ Add Peer</div>
      </div>
      <p className="text-[9px] text-gray-400 dark:text-gray-500 mb-2">A <span className="font-semibold text-gray-600 dark:text-gray-300">Peer</span> is a PostgreSQL connection — your source or destination database.</p>
      {[
        { name: 'production-db', host: 'pg-prod.internal', port: 5432, role: 'source' },
        { name: 'analytics-replica', host: 'pg-analytics.internal', port: 5432, role: 'destination' },
      ].map((peer, i) => (
        <div key={peer.name} className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Database className="w-3.5 h-3.5 text-blue-500" />
              <div>
                <span className="text-[11px] font-medium text-gray-900 dark:text-white">{peer.name}</span>
                <p className="text-[9px] text-gray-400">{peer.host}:{peer.port}</p>
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              {tested[i] ? (
                <span className="flex items-center gap-1 text-[9px] text-green-600 dark:text-green-400 font-medium">
                  <CheckCircle className="w-3 h-3" /> Connected
                </span>
              ) : (
                <span className="text-[9px] text-gray-400 animate-pulse">Testing...</span>
              )}
            </div>
          </div>
        </div>
      ))}
      <p className="text-[9px] text-gray-400 dark:text-gray-500 text-center pt-1">One-click test verifies credentials before you go live.</p>
    </div>
  );
}

function CreateMirrorScreen() {
  const [name, setName] = useState('');
  const [source, setSource] = useState('');
  const [dest, setDest] = useState('');

  useEffect(() => {
    const t1 = setTimeout(() => setName('prod-to-analytics'), 600);
    const t2 = setTimeout(() => setSource('production-db'), 1400);
    const t3 = setTimeout(() => setDest('analytics-replica'), 2200);
    return () => { clearTimeout(t1); clearTimeout(t2); clearTimeout(t3); };
  }, []);

  return (
    <div className="space-y-2.5">
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs font-semibold text-gray-900 dark:text-white">Create Mirror</span>
      </div>
      <p className="text-[9px] text-gray-400 dark:text-gray-500 mb-2">A <span className="font-semibold text-gray-600 dark:text-gray-300">Mirror</span> is a replication job — name it, pick a source peer and a destination peer.</p>
      <div className="space-y-2">
        <div className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-2.5">
          <div className="text-[9px] text-gray-400 mb-1">Mirror Name</div>
          <div className={`text-[11px] font-mono h-4 ${name ? 'text-gray-900 dark:text-white' : 'text-gray-300 dark:text-gray-700'}`}>
            {name || 'my-mirror'}
          </div>
        </div>
        <div className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-2.5">
          <div className="text-[9px] text-gray-400 mb-1">Source Peer</div>
          <div className="flex items-center gap-2">
            <Database className="w-3 h-3 text-blue-500" />
            <span className={`text-[11px] font-mono h-4 ${source ? 'text-gray-900 dark:text-white' : 'text-gray-300 dark:text-gray-700'}`}>
              {source || 'Select source...'}
            </span>
            {source && <CheckCircle className="w-3 h-3 text-green-500" />}
          </div>
        </div>
        <div className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-2.5">
          <div className="text-[9px] text-gray-400 mb-1">Destination Peer</div>
          <div className="flex items-center gap-2">
            <Database className="w-3 h-3 text-blue-500" />
            <span className={`text-[11px] font-mono h-4 ${dest ? 'text-gray-900 dark:text-white' : 'text-gray-300 dark:text-gray-700'}`}>
              {dest || 'Select destination...'}
            </span>
            {dest && <CheckCircle className="w-3 h-3 text-green-500" />}
          </div>
        </div>
      </div>
      <div className="flex items-center justify-end pt-1">
        <div className={`px-3 py-1 text-[9px] font-medium rounded transition-all ${
          name && source && dest ? 'bg-blue-500 text-white' : 'bg-gray-100 dark:bg-gray-800 text-gray-400'
        }`}>Next: Select Tables &rarr;</div>
      </div>
    </div>
  );
}

function TablesScreen() {
  const [checked, setChecked] = useState<number[]>([]);
  useEffect(() => {
    const timers = [0, 1, 2, 3, 4].map((i) =>
      setTimeout(() => setChecked((prev) => [...prev, i]), 400 + i * 350)
    );
    return () => timers.forEach(clearTimeout);
  }, []);

  const tables = ['users', 'orders', 'products', 'payments', 'sessions'];
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs font-semibold text-gray-900 dark:text-white">Select Tables</span>
        <span className="text-[9px] text-gray-400">{checked.length}/{tables.length} selected</span>
      </div>
      <p className="text-[9px] text-gray-400 dark:text-gray-500 mb-2">Pick which tables to replicate. A <span className="font-semibold text-gray-600 dark:text-gray-300">Mirror</span> streams changes from source peer to destination peer.</p>
      <div className="grid grid-cols-2 gap-1.5">
        {tables.map((t, i) => (
          <div key={t} className={`flex items-center gap-2 px-2.5 py-1.5 rounded border transition-all duration-200 ${
            checked.includes(i)
              ? 'border-blue-300 dark:border-blue-700 bg-blue-50 dark:bg-blue-950/30'
              : 'border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900'
          }`}>
            <div className={`w-3 h-3 rounded border-[1.5px] flex items-center justify-center transition-all ${
              checked.includes(i)
                ? 'border-blue-500 bg-blue-500'
                : 'border-gray-300 dark:border-gray-600'
            }`}>
              {checked.includes(i) && (
                <svg className="w-2 h-2 text-white" fill="currentColor" viewBox="0 0 12 12">
                  <path d="M10.28 2.28L3.989 8.575 1.695 6.28A1 1 0 00.28 7.695l3 3a1 1 0 001.414 0l7-7A1 1 0 0010.28 2.28z"/>
                </svg>
              )}
            </div>
            <span className="text-[10px] font-mono text-gray-700 dark:text-gray-300">public.{t}</span>
          </div>
        ))}
      </div>
      <div className="flex items-center justify-between pt-2 mt-1 border-t border-gray-100 dark:border-gray-800">
        <div className="text-[9px] text-gray-400">Source: production-db &rarr; Dest: analytics-replica</div>
        <div className={`px-2 py-0.5 text-[9px] font-medium rounded transition-all ${
          checked.length === 5 ? 'bg-blue-500 text-white' : 'bg-gray-100 dark:bg-gray-800 text-gray-400'
        }`}>Create Mirror</div>
      </div>
    </div>
  );
}

function MonitorScreen() {
  const [batch, setBatch] = useState(1243);
  const [rows, setRows] = useState(48721);
  useEffect(() => {
    const interval = setInterval(() => {
      setBatch((b) => b + Math.floor(Math.random() * 3) + 1);
      setRows((r) => r + Math.floor(Math.random() * 50) + 10);
    }, 1800);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="space-y-2.5">
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs font-semibold text-gray-900 dark:text-white">Mirrors</span>
        <div className="px-2 py-0.5 bg-blue-500 text-white text-[9px] font-medium rounded">+ Create Mirror</div>
      </div>
      <p className="text-[9px] text-gray-400 dark:text-gray-500 mb-2">A <span className="font-semibold text-gray-600 dark:text-gray-300">Mirror</span> is a live replication job — it continuously streams WAL changes between two peers.</p>
      {/* Active mirror */}
      <div className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-3">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
            <span className="text-[11px] font-semibold text-gray-900 dark:text-white">prod-to-analytics</span>
            <span className="text-[8px] font-medium text-green-700 dark:text-green-400 px-1.5 py-0.5 bg-green-100 dark:bg-green-900/30 rounded-full">REPLICATING</span>
          </div>
        </div>
        <div className="grid grid-cols-4 gap-2 mb-2">
          <div className="text-center">
            <div className="text-[8px] text-gray-400">Tables</div>
            <div className="text-[11px] font-bold text-gray-900 dark:text-white">5</div>
          </div>
          <div className="text-center">
            <div className="text-[8px] text-gray-400">Batches</div>
            <div className="text-[11px] font-bold text-gray-900 dark:text-white tabular-nums">{batch.toLocaleString()}</div>
          </div>
          <div className="text-center">
            <div className="text-[8px] text-gray-400">Rows</div>
            <div className="text-[11px] font-bold text-gray-900 dark:text-white tabular-nums">{rows.toLocaleString()}</div>
          </div>
          <div className="text-center">
            <div className="text-[8px] text-gray-400">Lag</div>
            <div className="text-[11px] font-bold text-green-600 dark:text-green-400">&lt;1s</div>
          </div>
        </div>
        <div className="h-1 bg-gray-100 dark:bg-gray-800 rounded-full overflow-hidden">
          <div className="h-full bg-gradient-to-r from-bunny-400 to-green-400 rounded-full animate-demo-flow" />
        </div>
      </div>
      {/* Paused */}
      <div className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-3 opacity-60">
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-yellow-500" />
          <span className="text-[11px] font-semibold text-gray-900 dark:text-white">staging-sync</span>
          <span className="text-[8px] font-medium text-yellow-700 dark:text-yellow-400 px-1.5 py-0.5 bg-yellow-100 dark:bg-yellow-900/30 rounded-full">PAUSED</span>
        </div>
      </div>
      {/* Log */}
      <div className="px-2.5 py-1.5 bg-gray-900 dark:bg-gray-800 rounded font-mono text-[9px] text-gray-300 flex items-center gap-1.5">
        <span className="text-green-400">INF</span>
        <span className="text-gray-500">15:04:32</span>
        <span className="truncate">batch #{batch} &middot; 23 rows &middot; LSN 0/1A3F{(batch % 1000).toString(16).toUpperCase().padStart(3, '0')}0</span>
      </div>
    </div>
  );
}

function ControlsScreen() {
  const [action, setAction] = useState<string | null>(null);
  useEffect(() => {
    const actions = ['pause', 'resume', 'resync', 'schema', null];
    let i = 0;
    const interval = setInterval(() => {
      setAction(actions[i % actions.length]);
      i++;
    }, 2000);
    return () => clearInterval(interval);
  }, []);

  const tables = [
    { name: 'public.users', rows: 12450, status: 'synced' },
    { name: 'public.orders', rows: 89321, status: 'synced' },
    { name: 'public.products', rows: 2100, status: action === 'resync' ? 'resyncing' : 'synced' },
  ];

  return (
    <div className="space-y-2.5">
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2">
          <span className="text-xs font-semibold text-gray-900 dark:text-white">prod-to-analytics</span>
          <span className="text-[8px] font-medium text-green-700 dark:text-green-400 px-1.5 py-0.5 bg-green-100 dark:bg-green-900/30 rounded-full">
            {action === 'pause' ? 'PAUSING...' : 'REPLICATING'}
          </span>
        </div>
      </div>
      <p className="text-[9px] text-gray-400 dark:text-gray-500 mb-2">Pause, resume, or resync individual tables without stopping the whole mirror.</p>
      {/* Action buttons */}
      <div className="flex items-center gap-1.5">
        {[
          { id: 'pause', icon: <Pause className="w-2.5 h-2.5" />, label: 'Pause' },
          { id: 'resume', icon: <Play className="w-2.5 h-2.5" />, label: 'Resume' },
          { id: 'resync', icon: <RefreshCw className="w-2.5 h-2.5" />, label: 'Resync' },
          { id: 'schema', icon: <FileText className="w-2.5 h-2.5" />, label: 'Sync Schema' },
        ].map((btn) => (
          <div key={btn.id} className={`flex items-center gap-1 px-2 py-1 rounded text-[9px] font-medium transition-all ${
            action === btn.id
              ? 'bg-bunny-100 dark:bg-bunny-900/30 text-bunny-700 dark:text-bunny-400 scale-105'
              : 'bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400'
          }`}>
            {btn.icon}{btn.label}
          </div>
        ))}
      </div>
      {/* Table status */}
      <div className="space-y-1">
        {tables.map((t) => (
          <div key={t.name} className="flex items-center justify-between px-2.5 py-1.5 bg-white dark:bg-gray-900 rounded border border-gray-200 dark:border-gray-800">
            <span className="text-[10px] font-mono text-gray-700 dark:text-gray-300">{t.name}</span>
            <div className="flex items-center gap-2">
              <span className="text-[9px] text-gray-400 tabular-nums">{t.rows.toLocaleString()} rows</span>
              {t.status === 'resyncing' ? (
                <span className="text-[8px] font-medium text-bunny-600 dark:text-bunny-400 animate-pulse">resyncing...</span>
              ) : (
                <CheckCircle className="w-3 h-3 text-green-500" />
              )}
            </div>
          </div>
        ))}
      </div>
      <p className="text-[9px] text-gray-400 dark:text-gray-500 text-center">Zero-downtime resync: shadow table → copy → atomic swap.</p>
    </div>
  );
}

function DemoAnimation() {
  const [activeScreen, setActiveScreen] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setActiveScreen((s) => (s + 1) % screens.length);
    }, 5000);
    return () => clearInterval(interval);
  }, []);

  const renderScreen = () => {
    switch (screens[activeScreen].id) {
      case 'peers': return <PeersScreen />;
      case 'create': return <CreateMirrorScreen />;
      case 'tables': return <TablesScreen />;
      case 'monitor': return <MonitorScreen />;
      case 'controls': return <ControlsScreen />;
    }
  };

  return (
    <div className="max-w-4xl mx-auto">
      <div className="rounded-xl border border-gray-200 dark:border-gray-800 overflow-hidden shadow-lg dark:shadow-gray-900/30">
        {/* Window chrome */}
        <div className="flex items-center gap-1.5 px-4 py-2 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <div className="w-2 h-2 rounded-full bg-red-400" />
          <div className="w-2 h-2 rounded-full bg-yellow-400" />
          <div className="w-2 h-2 rounded-full bg-green-400" />
          <div className="ml-3 flex-1 flex items-center justify-center">
            <div className="px-3 py-0.5 bg-gray-100 dark:bg-gray-800 rounded text-[10px] text-gray-400 font-mono">
              localhost:3000/{screens[activeScreen].route}
            </div>
          </div>
        </div>

        {/* Mock nav */}
        <div className="flex items-center justify-between px-4 py-1.5 border-b border-gray-100 dark:border-gray-800 bg-white dark:bg-gray-900">
          <div className="flex items-center gap-2">
            <BunnyLogo size={16} />
            <span className="text-[11px] font-semibold text-gray-900 dark:text-white">BunnyDB</span>
          </div>
          <div className="flex items-center gap-3 text-[10px] text-gray-400 dark:text-gray-500">
            <span className={['create', 'tables', 'monitor', 'controls'].includes(screens[activeScreen].id) ? 'text-bunny-600 dark:text-bunny-400 font-medium' : ''}>Mirrors</span>
            <span className={screens[activeScreen].id === 'peers' ? 'text-bunny-600 dark:text-bunny-400 font-medium' : ''}>Peers</span>
            <span>Settings</span>
          </div>
        </div>

        {/* Screen content */}
        <div className="bg-gray-50 dark:bg-gray-950 p-4 min-h-[280px]">
          {renderScreen()}
        </div>

        {/* Step tabs at bottom */}
        <div className="flex items-center justify-center gap-0.5 px-3 py-2.5 bg-white dark:bg-gray-900 border-t border-gray-100 dark:border-gray-800">
          {screens.map((s, i) => (
            <button
              key={s.id}
              onClick={() => setActiveScreen(i)}
              className={`flex items-center gap-1 px-2 py-1 rounded-md text-[9px] font-medium transition-all ${
                i === activeScreen
                  ? 'bg-gray-900 dark:bg-white text-white dark:text-gray-900'
                  : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              <span className="w-3 h-3 rounded-full border-[1.5px] flex items-center justify-center text-[7px] font-bold border-current">
                {i + 1}
              </span>
              {s.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-white dark:bg-gray-950">
      {/* Nav */}
      <nav className="sticky top-0 z-50 bg-white dark:bg-gray-900 shadow-sm dark:shadow-gray-900/20 border-b border-gray-200 dark:border-gray-800">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex items-center">
              <a href={basePath} className="flex items-center">
                <BunnyLogo size={28} />
                <span className="ml-2 text-xl font-bold text-gray-900 dark:text-white">BunnyDB</span>
              </a>
            </div>
            <div className="flex items-center space-x-4">
              <a
                href="https://github.com/Harshil-Jani/bunnyDB"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1.5 text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white"
              >
                <Github className="w-4 h-4" />
                GitHub
              </a>
              <a
                href={`${basePath}/docs`}
                className="flex items-center gap-1 text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white"
              >
                <BookOpen className="w-4 h-4" />
                Docs
              </a>
              <ThemeToggle />
            </div>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="max-w-6xl mx-auto px-6 pt-16 pb-12">
        <div className="text-center max-w-3xl mx-auto mb-12">
          <div className="flex items-center justify-center gap-2 mb-4">
            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-bunny-100 text-bunny-800 dark:bg-bunny-900 dark:text-bunny-200">
              Self-Hosted
            </span>
            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-200">
              Cloud Coming Soon
            </span>
          </div>
          <h1 className="text-4xl sm:text-5xl font-bold text-gray-900 dark:text-white tracking-tight leading-[1.15]">
            PostgreSQL replication<br />that just works.
          </h1>
          <p className="mt-5 text-lg text-gray-500 dark:text-gray-400 leading-relaxed max-w-xl mx-auto">
            Real-time Change Data Capture from one Postgres to another.
            Point, click, replicate. No configuration files, no CLI gymnastics.
          </p>
        </div>

        {/* Side by side: Demo + Self-Host */}
        <div className="grid lg:grid-cols-2 gap-6 items-start">
          {/* Demo */}
          <div>
            <div className="text-xs font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wider mb-3 text-center lg:text-left">
              See it in action
            </div>
            <DemoAnimation />
          </div>

          {/* Self-Host Terminal */}
          <div>
            <div className="text-xs font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wider mb-3 text-center lg:text-left">
              Deploy in 3 commands
            </div>
            <div className="rounded-xl border border-gray-200 dark:border-gray-800 overflow-hidden shadow-lg dark:shadow-gray-900/30 bg-gray-900 dark:bg-gray-950">
              {/* Terminal chrome */}
              <div className="flex items-center gap-1.5 px-4 py-2 border-b border-gray-800 bg-gray-900 dark:bg-gray-900">
                <div className="w-2 h-2 rounded-full bg-red-400" />
                <div className="w-2 h-2 rounded-full bg-yellow-400" />
                <div className="w-2 h-2 rounded-full bg-green-400" />
                <div className="ml-3 flex-1 flex items-center justify-center">
                  <div className="px-3 py-0.5 bg-gray-800 rounded text-[10px] text-gray-500 font-mono">
                    Terminal
                  </div>
                </div>
              </div>

              {/* Terminal content */}
              <div className="p-5 font-mono text-sm space-y-3">
                <div>
                  <span className="text-gray-500">$</span>
                  <span className="text-gray-300 ml-2">git clone https://github.com/Harshil-Jani/bunnyDB</span>
                </div>
                <div>
                  <span className="text-gray-500">$</span>
                  <span className="text-gray-300 ml-2">cd bunnyDB && make setup</span>
                </div>
                <div>
                  <span className="text-gray-500">$</span>
                  <span className="text-gray-300 ml-2">make up</span>
                </div>
                <div className="pt-2 border-t border-gray-800">
                  <span className="text-green-400">✓</span>
                  <span className="text-gray-400 ml-2">BunnyDB running at</span>
                  <span className="text-bunny-400 ml-1">localhost:3000</span>
                </div>
              </div>

              {/* CTA buttons */}
              <div className="px-5 pb-5 flex flex-wrap gap-2">
                <a
                  href={`${basePath}/docs/self-hosting`}
                  className="inline-flex items-center gap-2 px-4 py-2 bg-white text-gray-900 text-sm font-medium rounded-lg hover:bg-gray-100 transition-colors"
                >
                  Self-Hosting Guide
                  <ArrowRight className="w-3.5 h-3.5" />
                </a>
                <a
                  href="https://github.com/Harshil-Jani/bunnyDB"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-300 border border-gray-700 rounded-lg hover:border-gray-600 transition-colors"
                >
                  <Github className="w-3.5 h-3.5" />
                  GitHub
                </a>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* What BunnyDB does */}
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

          {/* Architecture flow */}
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

      {/* Features */}
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
