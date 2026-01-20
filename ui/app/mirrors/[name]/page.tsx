'use client';

import { useState, useEffect, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import {
  ArrowLeft,
  Play,
  Pause,
  RefreshCw,
  RotateCcw,
  Trash2,
  AlertCircle,
  CheckCircle,
  Clock,
  Database,
  Table,
  Activity,
  Settings,
  ChevronDown,
  ChevronUp,
  FileText,
  Info,
  AlertTriangle,
  XCircle,
  Plus,
  X,
  Save,
  Search,
} from 'lucide-react';

interface LogEntry {
  id: number;
  level: string;
  message: string;
  details?: string;
  created_at: string;
}

interface TableStatus {
  table_name: string;
  status: string;
  rows_synced: number;
  rows_inserted?: number;
  rows_updated?: number;
  last_synced_at?: string;
  error_message?: string;
}

interface MirrorDetails {
  name: string;
  status: string;
  slot_name: string;
  publication_name: string;
  last_lsn: number;
  last_sync_batch_id: number;
  error_message?: string;
  error_count: number;
  tables?: TableStatus[];
}

interface TableMapping {
  source_schema: string;
  source_table: string;
  destination_schema: string;
  destination_table: string;
  partition_key?: string;
  exclude_columns?: string[];
}

const API_URL = process.env.BUNNY_API_URL || 'http://localhost:8112';

export default function MirrorDetailPage() {
  const params = useParams();
  const router = useRouter();
  const mirrorName = params.name as string;

  const [mirror, setMirror] = useState<MirrorDetails | null>(null);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [expandedTables, setExpandedTables] = useState<Set<string>>(new Set());
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [activeTab, setActiveTab] = useState<'tables' | 'logs'>('tables');
  const [showTableEditor, setShowTableEditor] = useState(false);
  const [tableMappings, setTableMappings] = useState<TableMapping[]>([]);
  const [savingTables, setSavingTables] = useState(false);
  const [tableEditorError, setTableEditorError] = useState<string | null>(null);
  const [tableSearch, setTableSearch] = useState('');
  const [tableEditorSearch, setTableEditorSearch] = useState('');
  const [allTables, setAllTables] = useState<TableStatus[]>([]);
  const [availableTables, setAvailableTables] = useState<{ schema: string; table_name: string }[]>([]);
  const [loadingAvailableTables, setLoadingAvailableTables] = useState(false);

  const fetchMirrorDetails = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/v1/mirrors/${mirrorName}`);
      if (!res.ok) {
        if (res.status === 404) {
          setError('Mirror not found');
          return;
        }
        throw new Error('Failed to fetch mirror details');
      }
      const data = await res.json();
      setMirror(data);
      setError(null);
    } catch (err) {
      setError('Failed to connect to BunnyDB API');
    } finally {
      setLoading(false);
    }
  }, [mirrorName]);

  const fetchLogs = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/v1/mirrors/${mirrorName}/logs?limit=100`);
      if (res.ok) {
        const data = await res.json();
        setLogs(data || []);
      }
    } catch (err) {
      console.error('Failed to fetch logs:', err);
    }
  }, [mirrorName]);

  const [actionError, setActionError] = useState<string | null>(null);

  const performAction = async (action: string, tableName?: string) => {
    const actionKey = tableName ? `${action}-${tableName}` : action;
    setActionLoading(actionKey);
    setActionError(null);
    try {
      const url = tableName
        ? `${API_URL}/v1/mirrors/${mirrorName}/${action}/${tableName}`
        : `${API_URL}/v1/mirrors/${mirrorName}/${action}`;
      const res = await fetch(url, { method: 'POST' });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        if (res.status === 409) {
          // Operation already in progress
          setActionError(data.error || 'An operation is already in progress. Please wait for it to complete.');
          return;
        }
        throw new Error(data.error || `Failed to ${action}`);
      }
      await fetchMirrorDetails();
    } catch (err) {
      console.error(`Failed to ${action}:`, err);
      setActionError(err instanceof Error ? err.message : `Failed to ${action}`);
    } finally {
      setActionLoading(null);
    }
  };

  const deleteMirror = async () => {
    setActionLoading('delete');
    try {
      const res = await fetch(`${API_URL}/v1/mirrors/${mirrorName}`, {
        method: 'DELETE',
      });
      if (!res.ok) throw new Error('Failed to delete mirror');
      router.push('/');
    } catch (err) {
      console.error('Failed to delete:', err);
      setActionLoading(null);
    }
  };

  const fetchAllTables = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/v1/mirrors/${mirrorName}/tables`);
      if (res.ok) {
        const data = await res.json();
        if (data.tables && data.tables.length > 0) {
          setAllTables(data.tables);
        }
      }
    } catch (err) {
      console.error('Failed to fetch all tables:', err);
    }
  }, [mirrorName]);

  const fetchAvailableTables = useCallback(async () => {
    setLoadingAvailableTables(true);
    try {
      const res = await fetch(`${API_URL}/v1/mirrors/${mirrorName}/available-tables`);
      if (res.ok) {
        const data = await res.json();
        setAvailableTables(data || []);
      }
    } catch (err) {
      console.error('Failed to fetch available tables:', err);
    } finally {
      setLoadingAvailableTables(false);
    }
  }, [mirrorName]);

  useEffect(() => {
    fetchMirrorDetails();
    fetchLogs();
    fetchAllTables();
    const mirrorInterval = setInterval(fetchMirrorDetails, 5000);
    const logsInterval = setInterval(fetchLogs, 10000);
    const tablesInterval = setInterval(fetchAllTables, 10000);
    return () => {
      clearInterval(mirrorInterval);
      clearInterval(logsInterval);
      clearInterval(tablesInterval);
    };
  }, [fetchMirrorDetails, fetchLogs, fetchAllTables]);

  const toggleTableExpanded = (tableName: string) => {
    setExpandedTables((prev) => {
      const next = new Set(prev);
      if (next.has(tableName)) {
        next.delete(tableName);
      } else {
        next.add(tableName);
      }
      return next;
    });
  };

  const fetchTableMappings = async () => {
    try {
      const res = await fetch(`${API_URL}/v1/mirrors/${mirrorName}/tables`);
      if (res.ok) {
        const data = await res.json();
        if (data.config?.table_mappings && data.config.table_mappings.length > 0) {
          setTableMappings(data.config.table_mappings);
        } else if (data.tables && data.tables.length > 0) {
          // Create mappings from tables returned by API (from publication)
          setTableMappings(data.tables.map((t: { table_name: string }) => {
            const parts = t.table_name.split('.');
            return {
              source_schema: parts[0] || 'public',
              source_table: parts[1] || t.table_name,
              destination_schema: parts[0] || 'public',
              destination_table: parts[1] || t.table_name,
            };
          }));
        } else if (mirror?.tables && mirror.tables.length > 0) {
          // Fallback: create mappings from mirror state
          setTableMappings(mirror.tables.map(t => {
            const parts = t.table_name.split('.');
            return {
              source_schema: parts[0] || 'public',
              source_table: parts[1] || t.table_name,
              destination_schema: parts[0] || 'public',
              destination_table: parts[1] || t.table_name,
            };
          }));
        }
      }
    } catch (err) {
      console.error('Failed to fetch table mappings:', err);
    }
  };

  const openTableEditor = async () => {
    setTableEditorError(null);
    setTableEditorSearch('');
    await Promise.all([fetchTableMappings(), fetchAvailableTables()]);
    setShowTableEditor(true);
  };

  const addTableFromAvailable = (schema: string, tableName: string) => {
    // Check if already added
    const exists = tableMappings.some(
      (m) => m.source_schema === schema && m.source_table === tableName
    );
    if (exists) return;

    setTableMappings([...tableMappings, {
      source_schema: schema,
      source_table: tableName,
      destination_schema: schema,
      destination_table: tableName,
    }]);
    // Remove from available tables
    setAvailableTables(availableTables.filter(
      (t) => !(t.schema === schema && t.table_name === tableName)
    ));
  };

  const removeTableMapping = (index: number) => {
    const removed = tableMappings[index];
    setTableMappings(tableMappings.filter((_, i) => i !== index));
    // Add back to available tables
    if (removed) {
      setAvailableTables([...availableTables, {
        schema: removed.source_schema,
        table_name: removed.source_table,
      }].sort((a, b) => `${a.schema}.${a.table_name}`.localeCompare(`${b.schema}.${b.table_name}`)));
    }
  };

  const updateTableMapping = (index: number, field: keyof TableMapping, value: string) => {
    const updated = [...tableMappings];
    updated[index] = { ...updated[index], [field]: value };
    // Auto-fill destination if empty
    if (field === 'source_table' && !updated[index].destination_table) {
      updated[index].destination_table = value;
    }
    if (field === 'source_schema' && !updated[index].destination_schema) {
      updated[index].destination_schema = value;
    }
    setTableMappings(updated);
  };

  const saveTableMappings = async () => {
    if (tableMappings.length === 0) {
      setTableEditorError('At least one table mapping is required');
      return;
    }

    const invalid = tableMappings.find(t => !t.source_table || !t.destination_table);
    if (invalid) {
      setTableEditorError('All table names are required');
      return;
    }

    setSavingTables(true);
    setTableEditorError(null);
    try {
      const res = await fetch(`${API_URL}/v1/mirrors/${mirrorName}/tables`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ table_mappings: tableMappings }),
      });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to update tables');
      }
      setShowTableEditor(false);
      await Promise.all([fetchMirrorDetails(), fetchAllTables()]);
    } catch (err) {
      setTableEditorError(err instanceof Error ? err.message : 'Failed to save tables');
    } finally {
      setSavingTables(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status?.toUpperCase()) {
      case 'RUNNING':
      case 'SYNCED':
        return 'bg-green-100 text-green-800 border-green-200';
      case 'PAUSED':
      case 'PAUSING':
        return 'bg-yellow-100 text-yellow-800 border-yellow-200';
      case 'FAILED':
      case 'ERROR':
        return 'bg-red-100 text-red-800 border-red-200';
      case 'SNAPSHOT':
      case 'RESYNCING':
        return 'bg-blue-100 text-blue-800 border-blue-200';
      case 'SETTING_UP':
      case 'CREATED':
      case 'PENDING':
        return 'bg-gray-100 text-gray-800 border-gray-200';
      default:
        // CDC/SYNCING/Other processing states - use orange
        return 'bg-orange-100 text-orange-800 border-orange-200';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status?.toUpperCase()) {
      case 'RUNNING':
      case 'SYNCED':
        return <CheckCircle className="w-5 h-5 text-green-500" />;
      case 'PAUSED':
      case 'PAUSING':
        return <Pause className="w-5 h-5 text-yellow-500" />;
      case 'FAILED':
      case 'ERROR':
        return <AlertCircle className="w-5 h-5 text-red-500" />;
      case 'SNAPSHOT':
      case 'RESYNCING':
        return <RefreshCw className="w-5 h-5 text-blue-500 animate-spin" />;
      case 'SETTING_UP':
      case 'CREATED':
      case 'PENDING':
        return <Clock className="w-5 h-5 text-gray-500" />;
      default:
        // CDC/SYNCING/Other processing states - use orange with animation
        return <RefreshCw className="w-5 h-5 text-orange-500 animate-spin" />;
    }
  };

  const formatNumber = (num: number) => {
    return new Intl.NumberFormat().format(num);
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return 'Never';
    return new Date(dateStr).toLocaleString();
  };

  const getLogLevelIcon = (level: string) => {
    switch (level?.toUpperCase()) {
      case 'ERROR':
        return <XCircle className="w-4 h-4 text-red-500" />;
      case 'WARN':
        return <AlertTriangle className="w-4 h-4 text-yellow-500" />;
      case 'INFO':
        return <Info className="w-4 h-4 text-blue-500" />;
      default:
        return <Info className="w-4 h-4 text-gray-400" />;
    }
  };

  const getLogLevelColor = (level: string) => {
    switch (level?.toUpperCase()) {
      case 'ERROR':
        return 'bg-red-50 border-red-200';
      case 'WARN':
        return 'bg-yellow-50 border-yellow-200';
      case 'INFO':
        return 'bg-blue-50 border-blue-200';
      default:
        return 'bg-gray-50 border-gray-200';
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  if (error || !mirror) {
    return (
      <div className="space-y-6">
        <button
          onClick={() => router.push('/')}
          className="flex items-center gap-2 text-gray-600 hover:text-gray-900"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Mirrors
        </button>
        <div className="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
          <AlertCircle className="w-12 h-12 mx-auto text-red-500 mb-4" />
          <h2 className="text-lg font-semibold text-red-700">{error || 'Mirror not found'}</h2>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <button
            onClick={() => router.push('/')}
            className="flex items-center gap-2 text-gray-600 hover:text-gray-900"
          >
            <ArrowLeft className="w-4 h-4" />
          </button>
          <div className="flex items-center gap-3">
            {getStatusIcon(mirror.status)}
            <div>
              <h1 className="text-2xl font-bold text-gray-900">{mirror.name}</h1>
              <p className="text-sm text-gray-500">
                Slot: {mirror.slot_name || 'N/A'} | Publication: {mirror.publication_name || 'N/A'}
              </p>
            </div>
          </div>
        </div>
        <span className={`px-4 py-2 rounded-full text-sm font-medium border ${getStatusColor(mirror.status)}`}>
          {mirror.status}
        </span>
      </div>

      {/* Error Message */}
      {mirror.error_message && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <AlertCircle className="w-5 h-5 text-red-500 mt-0.5" />
            <div>
              <h3 className="font-medium text-red-800">Error</h3>
              <p className="text-sm text-red-700 mt-1">{mirror.error_message}</p>
              <p className="text-xs text-red-500 mt-2">Error count: {mirror.error_count}</p>
            </div>
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="bg-white rounded-lg shadow p-4">
        <h2 className="text-sm font-medium text-gray-500 mb-3">Actions</h2>
        {actionError && (
          <div className="mb-3 p-3 bg-amber-50 border border-amber-200 rounded-lg flex items-center gap-2">
            <AlertCircle className="w-4 h-4 text-amber-600 flex-shrink-0" />
            <span className="text-sm text-amber-800">{actionError}</span>
            <button
              onClick={() => setActionError(null)}
              className="ml-auto text-amber-600 hover:text-amber-800"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        )}
        <div className="flex flex-wrap gap-2">
          {!['PAUSED', 'PAUSING', 'TERMINATED', 'TERMINATING', 'FAILED'].includes(mirror.status?.toUpperCase()) && (
            <button
              onClick={() => performAction('pause')}
              disabled={actionLoading === 'pause'}
              className="flex items-center gap-2 px-4 py-2 bg-yellow-100 text-yellow-700 rounded-lg hover:bg-yellow-200 disabled:opacity-50"
            >
              {actionLoading === 'pause' ? (
                <RefreshCw className="w-4 h-4 animate-spin" />
              ) : (
                <Pause className="w-4 h-4" />
              )}
              Pause Mirror
            </button>
          )}
          {mirror.status?.toUpperCase() === 'PAUSED' && (
            <>
              <button
                onClick={() => performAction('resume')}
                disabled={actionLoading === 'resume'}
                className="flex items-center gap-2 px-4 py-2 bg-green-100 text-green-700 rounded-lg hover:bg-green-200 disabled:opacity-50"
              >
                {actionLoading === 'resume' ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <Play className="w-4 h-4" />
                )}
                Resume Mirror
              </button>
              <button
                onClick={openTableEditor}
                className="flex items-center gap-2 px-4 py-2 bg-cyan-100 text-cyan-700 rounded-lg hover:bg-cyan-200"
              >
                <Settings className="w-4 h-4" />
                Edit Tables
              </button>
            </>
          )}
          <button
            onClick={() => performAction('retry')}
            disabled={actionLoading === 'retry'}
            className="flex items-center gap-2 px-4 py-2 bg-blue-100 text-blue-700 rounded-lg hover:bg-blue-200 disabled:opacity-50"
          >
            {actionLoading === 'retry' ? (
              <RefreshCw className="w-4 h-4 animate-spin" />
            ) : (
              <RotateCcw className="w-4 h-4" />
            )}
            Retry Now
          </button>
          <button
            onClick={() => performAction('resync')}
            disabled={actionLoading === 'resync'}
            className="flex items-center gap-2 px-4 py-2 bg-purple-100 text-purple-700 rounded-lg hover:bg-purple-200 disabled:opacity-50"
          >
            {actionLoading === 'resync' ? (
              <RefreshCw className="w-4 h-4 animate-spin" />
            ) : (
              <RefreshCw className="w-4 h-4" />
            )}
            Full Resync
          </button>
          <button
            onClick={() => performAction('sync-schema')}
            disabled={actionLoading === 'sync-schema'}
            className="flex items-center gap-2 px-4 py-2 bg-indigo-100 text-indigo-700 rounded-lg hover:bg-indigo-200 disabled:opacity-50"
          >
            {actionLoading === 'sync-schema' ? (
              <RefreshCw className="w-4 h-4 animate-spin" />
            ) : (
              <Settings className="w-4 h-4" />
            )}
            Sync Schema
          </button>
          <button
            onClick={() => setShowDeleteConfirm(true)}
            className="flex items-center gap-2 px-4 py-2 bg-red-100 text-red-700 rounded-lg hover:bg-red-200 ml-auto"
          >
            <Trash2 className="w-4 h-4" />
            Delete Mirror
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-white rounded-lg shadow p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-blue-100 rounded-lg">
              <Activity className="w-5 h-5 text-blue-600" />
            </div>
            <div>
              <p className="text-sm text-gray-500">Last LSN</p>
              <p className="text-lg font-semibold font-mono">{formatNumber(mirror.last_lsn || 0)}</p>
            </div>
          </div>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-green-100 rounded-lg">
              <Database className="w-5 h-5 text-green-600" />
            </div>
            <div>
              <p className="text-sm text-gray-500">Sync Batch ID</p>
              <p className="text-lg font-semibold font-mono">{formatNumber(mirror.last_sync_batch_id || 0)}</p>
            </div>
          </div>
        </div>
        <div className="bg-white rounded-lg shadow p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-purple-100 rounded-lg">
              <Table className="w-5 h-5 text-purple-600" />
            </div>
            <div>
              <p className="text-sm text-gray-500">Tables</p>
              <p className="text-lg font-semibold">{allTables.length || mirror.tables?.length || 0}</p>
            </div>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="bg-white rounded-lg shadow">
        <div className="border-b">
          <nav className="flex">
            <button
              onClick={() => setActiveTab('tables')}
              className={`px-6 py-4 text-sm font-medium border-b-2 ${
                activeTab === 'tables'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              <div className="flex items-center gap-2">
                <Table className="w-4 h-4" />
                Tables ({allTables.length || mirror.tables?.length || 0})
              </div>
            </button>
            <button
              onClick={() => { setActiveTab('logs'); fetchLogs(); }}
              className={`px-6 py-4 text-sm font-medium border-b-2 ${
                activeTab === 'logs'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              <div className="flex items-center gap-2">
                <FileText className="w-4 h-4" />
                Logs ({logs.length})
              </div>
            </button>
          </nav>
        </div>

        {/* Tables Tab */}
        {activeTab === 'tables' && (
          <>
            {/* Search bar */}
            <div className="p-4 border-b">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search tables..."
                  value={tableSearch}
                  onChange={(e) => setTableSearch(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
            {(() => {
              const tablesToShow = allTables.length > 0 ? allTables : (mirror.tables || []);
              const filteredTables = tablesToShow.filter(t =>
                t.table_name.toLowerCase().includes(tableSearch.toLowerCase())
              );

              return filteredTables.length > 0 ? (
                <div className="divide-y max-h-[500px] overflow-y-auto">
                  {filteredTables.map((table) => (
                    <div key={table.table_name} className="p-4">
                      <div
                        className="flex items-center justify-between cursor-pointer"
                        onClick={() => toggleTableExpanded(table.table_name)}
                      >
                        <div className="flex items-center gap-3">
                          {expandedTables.has(table.table_name) ? (
                            <ChevronUp className="w-4 h-4 text-gray-400" />
                          ) : (
                            <ChevronDown className="w-4 h-4 text-gray-400" />
                          )}
                          <div>
                            <span className="font-mono text-sm font-medium">{table.table_name}</span>
                            <p className="text-xs text-gray-500">
                              {formatNumber(table.rows_synced)} rows synced
                              {(table.rows_inserted !== undefined || table.rows_updated !== undefined) && (
                                <span className="ml-2">
                                  ({formatNumber(table.rows_inserted || 0)} inserted, {formatNumber(table.rows_updated || 0)} updated)
                                </span>
                              )}
                            </p>
                          </div>
                        </div>
                        <div className="flex items-center gap-3">
                          <span className={`px-2 py-1 rounded text-xs font-medium ${getStatusColor(table.status)}`}>
                            {table.status}
                          </span>
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              performAction('resync', table.table_name);
                            }}
                            disabled={actionLoading === `resync-${table.table_name}`}
                            className="flex items-center gap-1 px-3 py-1.5 bg-purple-100 text-purple-700 rounded hover:bg-purple-200 disabled:opacity-50"
                            title="Resync this table"
                          >
                            {actionLoading === `resync-${table.table_name}` ? (
                              <RefreshCw className="w-3 h-3 animate-spin" />
                            ) : (
                              <RefreshCw className="w-3 h-3" />
                            )}
                            Resync
                          </button>
                        </div>
                      </div>
                      {expandedTables.has(table.table_name) && (
                        <div className="mt-4 ml-7 p-3 bg-gray-50 rounded-lg">
                          <div className="grid grid-cols-2 gap-4 text-sm">
                            <div>
                              <span className="text-gray-500">Status:</span>
                              <span className="ml-2 font-medium">{table.status}</span>
                            </div>
                            <div>
                              <span className="text-gray-500">Rows Synced:</span>
                              <span className="ml-2 font-medium font-mono">{formatNumber(table.rows_synced)}</span>
                            </div>
                            {table.rows_inserted !== undefined && (
                              <div>
                                <span className="text-gray-500">Rows Inserted:</span>
                                <span className="ml-2 font-medium font-mono text-green-600">{formatNumber(table.rows_inserted)}</span>
                              </div>
                            )}
                            {table.rows_updated !== undefined && (
                              <div>
                                <span className="text-gray-500">Rows Updated:</span>
                                <span className="ml-2 font-medium font-mono text-blue-600">{formatNumber(table.rows_updated)}</span>
                              </div>
                            )}
                            <div>
                              <span className="text-gray-500">Last Synced:</span>
                              <span className="ml-2 font-medium">{formatDate(table.last_synced_at)}</span>
                            </div>
                            {table.error_message && (
                              <div className="col-span-2">
                                <span className="text-red-500">Error:</span>
                                <span className="ml-2 text-red-700">{table.error_message}</span>
                              </div>
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <div className="p-8 text-center text-gray-500">
                  <Table className="w-12 h-12 mx-auto text-gray-300 mb-3" />
                  {tableSearch ? (
                    <p>No tables matching &quot;{tableSearch}&quot;</p>
                  ) : (
                    <>
                      <p>No table status information available yet.</p>
                      <p className="text-sm mt-1">Table status will appear once the mirror starts syncing.</p>
                    </>
                  )}
                </div>
              );
            })()}
          </>
        )}

        {/* Logs Tab */}
        {activeTab === 'logs' && (
          <div className="divide-y max-h-[600px] overflow-y-auto">
            {logs.length > 0 ? (
              logs.map((log) => (
                <div key={log.id} className={`p-4 ${getLogLevelColor(log.level)} border-l-4`}>
                  <div className="flex items-start gap-3">
                    {getLogLevelIcon(log.level)}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between gap-4">
                        <span className="font-medium text-gray-900">{log.message}</span>
                        <span className="text-xs text-gray-500 whitespace-nowrap">
                          {formatDate(log.created_at)}
                        </span>
                      </div>
                      {log.details && (
                        <pre className="mt-2 text-xs text-gray-600 bg-white bg-opacity-50 rounded p-2 overflow-x-auto">
                          {(() => {
                            try {
                              return JSON.stringify(JSON.parse(log.details), null, 2);
                            } catch {
                              return log.details;
                            }
                          })()}
                        </pre>
                      )}
                    </div>
                  </div>
                </div>
              ))
            ) : (
              <div className="p-8 text-center text-gray-500">
                <FileText className="w-12 h-12 mx-auto text-gray-300 mb-3" />
                <p>No logs available yet.</p>
                <p className="text-sm mt-1">Logs will appear as the mirror performs operations.</p>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Delete Confirmation Modal */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-lg shadow-xl max-w-md w-full p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-2">Delete Mirror?</h3>
            <p className="text-gray-600 mb-4">
              Are you sure you want to delete &quot;{mirror.name}&quot;? This will stop replication and clean up
              the replication slot and publication on the source database.
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowDeleteConfirm(false)}
                className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={deleteMirror}
                disabled={actionLoading === 'delete'}
                className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50"
              >
                {actionLoading === 'delete' ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <Trash2 className="w-4 h-4" />
                )}
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Table Editor Modal */}
      {showTableEditor && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-lg shadow-xl max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col">
            <div className="p-6 border-b flex items-center justify-between">
              <div>
                <h3 className="text-lg font-semibold text-gray-900">Edit Table Mappings</h3>
                <p className="text-sm text-gray-500 mt-1">Add or remove tables from this mirror while paused</p>
              </div>
              <button
                onClick={() => setShowTableEditor(false)}
                className="p-2 hover:bg-gray-100 rounded-lg"
              >
                <X className="w-5 h-5 text-gray-500" />
              </button>
            </div>

            <div className="p-6 overflow-y-auto flex-1">
              {tableEditorError && (
                <div className="mb-4 bg-red-50 border border-red-200 rounded-lg p-3 flex items-center gap-2">
                  <AlertCircle className="w-4 h-4 text-red-500" />
                  <span className="text-sm text-red-700">{tableEditorError}</span>
                </div>
              )}

              {/* Search bar for table editor */}
              <div className="mb-4 relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search tables..."
                  value={tableEditorSearch}
                  onChange={(e) => setTableEditorSearch(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <span className="absolute right-3 top-1/2 transform -translate-y-1/2 text-xs text-gray-400">
                  {tableMappings.length} tables
                </span>
              </div>

              <div className="space-y-4 max-h-[400px] overflow-y-auto">
                {tableMappings
                  .map((mapping, index) => ({ mapping, index }))
                  .filter(({ mapping }) =>
                    tableEditorSearch === '' ||
                    mapping.source_table.toLowerCase().includes(tableEditorSearch.toLowerCase()) ||
                    mapping.source_schema.toLowerCase().includes(tableEditorSearch.toLowerCase()) ||
                    mapping.destination_table.toLowerCase().includes(tableEditorSearch.toLowerCase())
                  )
                  .map(({ mapping, index }) => (
                  <div key={index} className="bg-gray-50 rounded-lg p-4 border">
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex-1 grid grid-cols-2 gap-4">
                        <div>
                          <label className="block text-xs font-medium text-gray-500 mb-1">Source Schema</label>
                          <input
                            type="text"
                            value={mapping.source_schema}
                            onChange={(e) => updateTableMapping(index, 'source_schema', e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg text-sm font-mono"
                            placeholder="public"
                          />
                        </div>
                        <div>
                          <label className="block text-xs font-medium text-gray-500 mb-1">Source Table</label>
                          <input
                            type="text"
                            value={mapping.source_table}
                            onChange={(e) => updateTableMapping(index, 'source_table', e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg text-sm font-mono"
                            placeholder="table_name"
                          />
                        </div>
                        <div>
                          <label className="block text-xs font-medium text-gray-500 mb-1">Destination Schema</label>
                          <input
                            type="text"
                            value={mapping.destination_schema}
                            onChange={(e) => updateTableMapping(index, 'destination_schema', e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg text-sm font-mono"
                            placeholder="public"
                          />
                        </div>
                        <div>
                          <label className="block text-xs font-medium text-gray-500 mb-1">Destination Table</label>
                          <input
                            type="text"
                            value={mapping.destination_table}
                            onChange={(e) => updateTableMapping(index, 'destination_table', e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg text-sm font-mono"
                            placeholder="table_name"
                          />
                        </div>
                      </div>
                      <button
                        onClick={() => removeTableMapping(index)}
                        className="p-2 text-red-500 hover:bg-red-50 rounded-lg mt-5"
                        title="Remove table"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                ))}

                {tableMappings.length === 0 && (
                  <div className="text-center py-8 text-gray-500">
                    <Table className="w-12 h-12 mx-auto text-gray-300 mb-3" />
                    <p>No tables configured</p>
                    <p className="text-sm mt-1">Add tables to replicate</p>
                  </div>
                )}

                {tableMappings.length > 0 && tableEditorSearch && tableMappings.filter(m =>
                  m.source_table.toLowerCase().includes(tableEditorSearch.toLowerCase()) ||
                  m.source_schema.toLowerCase().includes(tableEditorSearch.toLowerCase())
                ).length === 0 && (
                  <div className="text-center py-8 text-gray-500">
                    <Search className="w-12 h-12 mx-auto text-gray-300 mb-3" />
                    <p>No tables matching &quot;{tableEditorSearch}&quot;</p>
                  </div>
                )}
              </div>

              {/* Available Tables Section */}
              <div className="mt-6 border-t pt-4">
                <h4 className="text-sm font-medium text-gray-700 mb-3 flex items-center gap-2">
                  <Plus className="w-4 h-4" />
                  Add Tables from Source Database
                </h4>
                {loadingAvailableTables ? (
                  <div className="flex items-center justify-center py-4">
                    <RefreshCw className="w-5 h-5 animate-spin text-blue-500" />
                    <span className="ml-2 text-sm text-gray-500">Loading available tables...</span>
                  </div>
                ) : availableTables.length > 0 ? (
                  <>
                    <div className="max-h-[200px] overflow-y-auto border rounded-lg divide-y">
                      {availableTables
                        .filter(t =>
                          tableEditorSearch === '' ||
                          t.table_name.toLowerCase().includes(tableEditorSearch.toLowerCase()) ||
                          t.schema.toLowerCase().includes(tableEditorSearch.toLowerCase())
                        )
                        .map((table) => (
                          <div
                            key={`${table.schema}.${table.table_name}`}
                            className="flex items-center justify-between px-3 py-2 hover:bg-gray-50"
                          >
                            <span className="font-mono text-sm">
                              {table.schema}.{table.table_name}
                            </span>
                            <button
                              onClick={() => addTableFromAvailable(table.schema, table.table_name)}
                              className="flex items-center gap-1 px-2 py-1 text-xs bg-green-100 text-green-700 rounded hover:bg-green-200"
                            >
                              <Plus className="w-3 h-3" />
                              Add
                            </button>
                          </div>
                        ))}
                      {availableTables.filter(t =>
                        tableEditorSearch === '' ||
                        t.table_name.toLowerCase().includes(tableEditorSearch.toLowerCase()) ||
                        t.schema.toLowerCase().includes(tableEditorSearch.toLowerCase())
                      ).length === 0 && (
                        <div className="text-center py-4 text-gray-500 text-sm">
                          No available tables matching search
                        </div>
                      )}
                    </div>
                    <p className="text-xs text-gray-400 mt-2">
                      {availableTables.length} table{availableTables.length !== 1 ? 's' : ''} available to add
                    </p>
                  </>
                ) : (
                  <div className="text-center py-4 text-gray-500 text-sm border rounded-lg bg-gray-50">
                    <CheckCircle className="w-5 h-5 mx-auto text-green-500 mb-1" />
                    All source tables are already in the mirror
                  </div>
                )}
              </div>
            </div>

            <div className="p-6 border-t bg-gray-50 flex justify-end gap-3">
              <button
                onClick={() => setShowTableEditor(false)}
                className="px-4 py-2 text-gray-700 hover:bg-gray-200 rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={saveTableMappings}
                disabled={savingTables}
                className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {savingTables ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <Save className="w-4 h-4" />
                )}
                Save Changes
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
