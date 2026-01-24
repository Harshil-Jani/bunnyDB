'use client';

import React, { useState, useEffect, useCallback, useRef } from 'react';
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
  Plus,
  X,
  Save,
  Search,
  Info,
} from 'lucide-react';
import { getStatusColor, getStatusIcon, getLogEventCategory, getEventCategoryColor, getEventCategoryBadge, getEventCategoryIcon, getEventCategoryFilterColor, LOG_EVENT_CATEGORIES, LogEventCategory } from '../../../lib/status';

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
  const [logsTotal, setLogsTotal] = useState(0);
  const [logsOffset, setLogsOffset] = useState(0);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logSearch, setLogSearch] = useState('');
  const [logSearchInput, setLogSearchInput] = useState('');
  const [logEventFilter, setLogEventFilter] = useState<LogEventCategory | ''>('');
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
  const LOGS_PAGE_SIZE = 50;

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

  const logsOffsetRef = useRef(logsOffset);
  logsOffsetRef.current = logsOffset;

  const fetchLogs = useCallback(async (append = false, overrideOffset?: number) => {
    setLogsLoading(true);
    try {
      const currentOffset = overrideOffset ?? (append ? logsOffsetRef.current : 0);
      const params = new URLSearchParams({
        limit: String(LOGS_PAGE_SIZE),
        offset: String(currentOffset),
      });
      if (logSearch) params.set('search', logSearch);

      const res = await fetch(`${API_URL}/v1/mirrors/${mirrorName}/logs?${params}`);
      if (res.ok) {
        const data = await res.json();
        const newLogs = data.logs || [];
        if (append) {
          setLogs(prev => [...prev, ...newLogs]);
        } else {
          setLogs(newLogs);
        }
        setLogsTotal(data.total || 0);
        const newOffset = currentOffset + newLogs.length;
        setLogsOffset(newOffset);
        logsOffsetRef.current = newOffset;
      }
    } catch (err) {
      console.error('Failed to fetch logs:', err);
    } finally {
      setLogsLoading(false);
    }
  }, [mirrorName, logSearch]);

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
    const tablesInterval = setInterval(fetchAllTables, 10000);
    return () => {
      clearInterval(mirrorInterval);
      clearInterval(tablesInterval);
    };
  }, [fetchMirrorDetails, fetchLogs, fetchAllTables]);

  // Re-fetch logs when search changes (reset to page 1)
  useEffect(() => {
    setLogs([]);
    setLogsOffset(0);
    fetchLogs(false, 0);
  }, [logSearch]); // eslint-disable-line react-hooks/exhaustive-deps

  // Filter logs client-side by event category
  const filteredLogs = logEventFilter
    ? logs.filter(log => getLogEventCategory(log.message, log.level) === logEventFilter)
    : logs;

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

  const formatNumber = (num: number) => {
    return new Intl.NumberFormat().format(num);
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return 'Never';
    return new Date(dateStr).toLocaleString();
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
          className="flex items-center gap-2 text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Mirrors
        </button>
        <div className="bg-red-50 border border-red-200 rounded-lg p-6 text-center dark:bg-red-900/20 dark:border-red-800">
          <AlertCircle className="w-12 h-12 mx-auto text-red-500 mb-4" />
          <h2 className="text-lg font-semibold text-red-700 dark:text-red-400">{error || 'Mirror not found'}</h2>
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
            className="flex items-center gap-2 text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white"
          >
            <ArrowLeft className="w-4 h-4" />
          </button>
          <div className="flex items-center gap-3">
            {getStatusIcon(mirror.status, 'md')}
            <div>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{mirror.name}</h1>
              <p className="text-sm text-gray-500 dark:text-gray-400">
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
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 dark:bg-red-900/20 dark:border-red-800">
          <div className="flex items-start gap-3">
            <AlertCircle className="w-5 h-5 text-red-500 mt-0.5" />
            <div>
              <h3 className="font-medium text-red-800 dark:text-red-400">Error</h3>
              <p className="text-sm text-red-700 dark:text-red-400 mt-1">{mirror.error_message}</p>
              <p className="text-xs text-red-500 dark:text-red-400 mt-2">Error count: {mirror.error_count}</p>
            </div>
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-4">
        <h2 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-3">Actions</h2>
        {actionError && (
          <div className="mb-3 p-3 bg-amber-50 border border-amber-200 rounded-lg flex items-center gap-2 dark:bg-amber-900/20 dark:border-amber-800">
            <AlertCircle className="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0" />
            <span className="text-sm text-amber-800 dark:text-amber-300">{actionError}</span>
            <button
              onClick={() => setActionError(null)}
              className="ml-auto text-amber-600 hover:text-amber-800 dark:text-amber-400 dark:hover:text-amber-200"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        )}
        <div className="flex flex-wrap gap-2">
          {!['PAUSED', 'PAUSING', 'TERMINATED', 'TERMINATING', 'FAILED'].includes(mirror.status?.toUpperCase()) && (
            <div className="relative group/pause">
              <button
                onClick={() => performAction('pause')}
                disabled={actionLoading === 'pause'}
                className="flex items-center gap-2 px-4 py-2 bg-yellow-100 text-yellow-700 rounded-lg hover:bg-yellow-200 disabled:opacity-50 dark:bg-yellow-900/30 dark:text-yellow-400 dark:hover:bg-yellow-900/50"
              >
                {actionLoading === 'pause' ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <Pause className="w-4 h-4" />
                )}
                Pause Mirror
                <Info className="w-3.5 h-3.5 opacity-50" />
              </button>
              <div className="absolute bottom-full left-0 mb-2 w-64 p-2.5 bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover/pause:opacity-100 group-hover/pause:visible transition-all duration-150 z-50 pointer-events-none">
                Stops CDC replication without dropping the replication slot. Changes on the source accumulate in the WAL and will be replayed when resumed. No data loss.
                <div className="absolute top-full left-4 border-4 border-transparent border-t-gray-900 dark:border-t-gray-700"></div>
              </div>
            </div>
          )}
          {mirror.status?.toUpperCase() === 'PAUSED' && (
            <>
              <div className="relative group/resume">
                <button
                  onClick={() => performAction('resume')}
                  disabled={actionLoading === 'resume'}
                  className="flex items-center gap-2 px-4 py-2 bg-green-100 text-green-700 rounded-lg hover:bg-green-200 disabled:opacity-50 dark:bg-green-900/30 dark:text-green-400 dark:hover:bg-green-900/50"
                >
                  {actionLoading === 'resume' ? (
                    <RefreshCw className="w-4 h-4 animate-spin" />
                  ) : (
                    <Play className="w-4 h-4" />
                  )}
                  Resume Mirror
                  <Info className="w-3.5 h-3.5 opacity-50" />
                </button>
                <div className="absolute bottom-full left-0 mb-2 w-64 p-2.5 bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover/resume:opacity-100 group-hover/resume:visible transition-all duration-150 z-50 pointer-events-none">
                  Resumes CDC replication from where it left off. All accumulated WAL changes will be replayed to catch up the destination.
                  <div className="absolute top-full left-4 border-4 border-transparent border-t-gray-900 dark:border-t-gray-700"></div>
                </div>
              </div>
              <button
                onClick={openTableEditor}
                className="flex items-center gap-2 px-4 py-2 bg-cyan-100 text-cyan-700 rounded-lg hover:bg-cyan-200 dark:bg-cyan-900/30 dark:text-cyan-400 dark:hover:bg-cyan-900/50"
              >
                <Settings className="w-4 h-4" />
                Edit Tables
              </button>
            </>
          )}
          <div className="relative group/retry">
            <button
              onClick={() => performAction('retry')}
              disabled={actionLoading === 'retry'}
              className="flex items-center gap-2 px-4 py-2 bg-blue-100 text-blue-700 rounded-lg hover:bg-blue-200 disabled:opacity-50 dark:bg-blue-900/30 dark:text-blue-400 dark:hover:bg-blue-900/50"
            >
              {actionLoading === 'retry' ? (
                <RefreshCw className="w-4 h-4 animate-spin" />
              ) : (
                <RotateCcw className="w-4 h-4" />
              )}
              Retry Now
              <Info className="w-3.5 h-3.5 opacity-50" />
            </button>
            <div className="absolute bottom-full left-0 mb-2 w-72 p-2.5 bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover/retry:opacity-100 group-hover/retry:visible transition-all duration-150 z-50 pointer-events-none">
              Bypasses the error backoff timer and immediately restarts CDC. Drops the replication slot and creates a fresh connection. Use when the underlying issue is fixed.
              <div className="absolute top-full left-4 border-4 border-transparent border-t-gray-900 dark:border-t-gray-700"></div>
            </div>
          </div>
          <div className="relative group/resync">
            <button
              onClick={() => performAction('resync')}
              disabled={actionLoading === 'resync'}
              className="flex items-center gap-2 px-4 py-2 bg-purple-100 text-purple-700 rounded-lg hover:bg-purple-200 disabled:opacity-50 dark:bg-purple-900/30 dark:text-purple-400 dark:hover:bg-purple-900/50"
            >
              {actionLoading === 'resync' ? (
                <RefreshCw className="w-4 h-4 animate-spin" />
              ) : (
                <RefreshCw className="w-4 h-4" />
              )}
              Full Resync
              <Info className="w-3.5 h-3.5 opacity-50" />
            </button>
            <div className="absolute bottom-full left-0 mb-2 w-72 p-2.5 bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover/resync:opacity-100 group-hover/resync:visible transition-all duration-150 z-50 pointer-events-none">
              Re-copies all table data from source to destination. Uses the swap strategy: creates shadow tables, copies data, then atomically renames them into place for zero downtime. Resets the replication slot after completion.
              <div className="absolute top-full left-4 border-4 border-transparent border-t-gray-900 dark:border-t-gray-700"></div>
            </div>
          </div>
          <div className="relative group/schema">
            <button
              onClick={() => performAction('sync-schema')}
              disabled={actionLoading === 'sync-schema'}
              className="flex items-center gap-2 px-4 py-2 bg-indigo-100 text-indigo-700 rounded-lg hover:bg-indigo-200 disabled:opacity-50 dark:bg-indigo-900/30 dark:text-indigo-400 dark:hover:bg-indigo-900/50"
            >
              {actionLoading === 'sync-schema' ? (
                <RefreshCw className="w-4 h-4 animate-spin" />
              ) : (
                <Settings className="w-4 h-4" />
              )}
              Sync Schema
              <Info className="w-3.5 h-3.5 opacity-50" />
            </button>
            <div className="absolute bottom-full left-0 mb-2 w-72 p-2.5 bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover/schema:opacity-100 group-hover/schema:visible transition-all duration-150 z-50 pointer-events-none">
              Compares source and destination table schemas, then applies DDL changes (new columns, type alterations) to the destination. Restarts CDC to pick up the new column definitions. Does not re-copy data.
              <div className="absolute top-full left-4 border-4 border-transparent border-t-gray-900 dark:border-t-gray-700"></div>
            </div>
          </div>
          <div className="relative group/delete ml-auto">
            <button
              onClick={() => setShowDeleteConfirm(true)}
              className="flex items-center gap-2 px-4 py-2 bg-red-100 text-red-700 rounded-lg hover:bg-red-200 dark:bg-red-900/30 dark:text-red-400 dark:hover:bg-red-900/50"
            >
              <Trash2 className="w-4 h-4" />
              Delete Mirror
              <Info className="w-3.5 h-3.5 opacity-50" />
            </button>
            <div className="absolute bottom-full right-0 mb-2 w-64 p-2.5 bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover/delete:opacity-100 group-hover/delete:visible transition-all duration-150 z-50 pointer-events-none">
              Permanently removes this mirror. Drops the replication slot and publication on the source. Destination tables are kept intact.
              <div className="absolute top-full right-4 border-4 border-transparent border-t-gray-900 dark:border-t-gray-700"></div>
            </div>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
              <Activity className="w-5 h-5 text-blue-600 dark:text-blue-400" />
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Last LSN</p>
              <p className="text-lg font-semibold font-mono">{formatNumber(mirror.last_lsn || 0)}</p>
            </div>
          </div>
        </div>
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-green-100 dark:bg-green-900/30 rounded-lg">
              <Database className="w-5 h-5 text-green-600 dark:text-green-400" />
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Sync Batch ID</p>
              <p className="text-lg font-semibold font-mono">{formatNumber(mirror.last_sync_batch_id || 0)}</p>
            </div>
          </div>
        </div>
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-purple-100 dark:bg-purple-900/30 rounded-lg">
              <Table className="w-5 h-5 text-purple-600 dark:text-purple-400" />
            </div>
            <div>
              <p className="text-sm text-gray-500 dark:text-gray-400">Tables</p>
              <p className="text-lg font-semibold">{allTables.length || mirror.tables?.length || 0}</p>
            </div>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20">
        <div className="border-b dark:border-gray-700">
          <nav className="flex">
            <button
              onClick={() => setActiveTab('tables')}
              className={`px-6 py-4 text-sm font-medium border-b-2 ${
                activeTab === 'tables'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:border-gray-600'
              }`}
            >
              <div className="flex items-center gap-2">
                <Table className="w-4 h-4" />
                Tables ({allTables.length || mirror.tables?.length || 0})
              </div>
            </button>
            <button
              onClick={() => { setActiveTab('logs'); setLogs([]); setLogsOffset(0); fetchLogs(false, 0); }}
              className={`px-6 py-4 text-sm font-medium border-b-2 ${
                activeTab === 'logs'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:border-gray-600'
              }`}
            >
              <div className="flex items-center gap-2">
                <FileText className="w-4 h-4" />
                Logs {logsTotal > 0 && <span className="text-xs bg-gray-200 dark:bg-gray-700 px-1.5 py-0.5 rounded-full">{logsTotal}</span>}
              </div>
            </button>
          </nav>
        </div>

        {/* Tables Tab */}
        {activeTab === 'tables' && (
          <>
            {/* Search bar */}
            <div className="p-4 border-b dark:border-gray-700">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search tables..."
                  value={tableSearch}
                  onChange={(e) => setTableSearch(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
                />
              </div>
            </div>
            {(() => {
              const tablesToShow = allTables.length > 0 ? allTables : (mirror.tables || []);
              const filteredTables = tablesToShow.filter(t =>
                t.table_name.toLowerCase().includes(tableSearch.toLowerCase())
              );

              return filteredTables.length > 0 ? (
                <div className="divide-y dark:divide-gray-700 max-h-[500px] overflow-y-auto">
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
                            <p className="text-xs text-gray-500 dark:text-gray-400">
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
                            className="flex items-center gap-1 px-3 py-1.5 bg-purple-100 text-purple-700 rounded hover:bg-purple-200 disabled:opacity-50 dark:bg-purple-900/30 dark:text-purple-400 dark:hover:bg-purple-900/50"
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
                        <div className="mt-4 ml-7 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                          <div className="grid grid-cols-2 gap-4 text-sm">
                            <div>
                              <span className="text-gray-500 dark:text-gray-400">Status:</span>
                              <span className="ml-2 font-medium">{table.status}</span>
                            </div>
                            <div>
                              <span className="text-gray-500 dark:text-gray-400">Rows Synced:</span>
                              <span className="ml-2 font-medium font-mono">{formatNumber(table.rows_synced)}</span>
                            </div>
                            {table.rows_inserted !== undefined && (
                              <div>
                                <span className="text-gray-500 dark:text-gray-400">Rows Inserted:</span>
                                <span className="ml-2 font-medium font-mono text-green-600 dark:text-green-400">{formatNumber(table.rows_inserted)}</span>
                              </div>
                            )}
                            {table.rows_updated !== undefined && (
                              <div>
                                <span className="text-gray-500 dark:text-gray-400">Rows Updated:</span>
                                <span className="ml-2 font-medium font-mono text-blue-600 dark:text-blue-400">{formatNumber(table.rows_updated)}</span>
                              </div>
                            )}
                            <div>
                              <span className="text-gray-500 dark:text-gray-400">Last Synced:</span>
                              <span className="ml-2 font-medium">{formatDate(table.last_synced_at)}</span>
                            </div>
                            {table.error_message && (
                              <div className="col-span-2">
                                <span className="text-red-500">Error:</span>
                                <span className="ml-2 text-red-700 dark:text-red-400">{table.error_message}</span>
                              </div>
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <div className="p-8 text-center text-gray-500 dark:text-gray-400">
                  <Table className="w-12 h-12 mx-auto text-gray-300 dark:text-gray-600 mb-3" />
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
          <div>
            {/* Search Bar */}
            <div className="p-4 border-b dark:border-gray-700 flex flex-wrap items-center gap-3">
              <div className="relative flex-1 min-w-[200px]">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input
                  type="text"
                  placeholder="Search logs..."
                  value={logSearchInput}
                  onChange={(e) => setLogSearchInput(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') setLogSearch(logSearchInput); }}
                  className="w-full pl-10 pr-4 py-2 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
                />
              </div>
              {logSearchInput && logSearch !== logSearchInput && (
                <button
                  onClick={() => setLogSearch(logSearchInput)}
                  className="px-3 py-2 text-xs bg-blue-100 text-blue-700 rounded-lg hover:bg-blue-200 dark:bg-blue-900/30 dark:text-blue-400"
                >
                  Search
                </button>
              )}
              {logSearch && (
                <button
                  onClick={() => { setLogSearch(''); setLogSearchInput(''); }}
                  className="px-3 py-2 text-xs bg-gray-100 text-gray-600 rounded-lg hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300"
                >
                  Clear
                </button>
              )}
              <span className="text-xs text-gray-500 dark:text-gray-400 ml-auto">
                {logEventFilter ? `${filteredLogs.length} shown` : `${logs.length} of ${logsTotal}`}
              </span>
            </div>

            {/* Event Category Filters */}
            <div className="px-4 py-2.5 border-b dark:border-gray-700 flex flex-wrap items-center gap-1.5">
              <span className="text-xs text-gray-500 dark:text-gray-400 mr-1">Events:</span>
              <button
                onClick={() => setLogEventFilter('')}
                className={`px-2.5 py-1 text-xs rounded-md font-medium transition-colors ${
                  logEventFilter === '' ? 'bg-gray-200 text-gray-800 dark:bg-gray-600 dark:text-white' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700'
                }`}
              >
                All
              </button>
              {LOG_EVENT_CATEGORIES.map((cat) => (
                <button
                  key={cat.key}
                  onClick={() => setLogEventFilter(logEventFilter === cat.key ? '' : cat.key)}
                  className={`px-2.5 py-1 text-xs rounded-md font-medium transition-colors ${
                    getEventCategoryFilterColor(cat.key, logEventFilter === cat.key)
                  }`}
                >
                  {cat.label}
                </button>
              ))}
            </div>

            {/* Log Entries */}
            <div className="max-h-[600px] overflow-y-auto">
              {filteredLogs.length > 0 ? (
                <>
                  {filteredLogs.map((log) => {
                    const category = getLogEventCategory(log.message, log.level);
                    return (
                      <div key={log.id} className={`px-4 py-3 border-l-4 border-b dark:border-b-gray-700/50 ${getEventCategoryColor(category)}`}>
                        <div className="flex items-start gap-3">
                          {getEventCategoryIcon(category)}
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 flex-wrap">
                              <span className={`px-1.5 py-0.5 rounded text-[10px] font-bold uppercase ${getEventCategoryBadge(category)}`}>
                                {LOG_EVENT_CATEGORIES.find(c => c.key === category)?.label || category}
                              </span>
                              <span className="font-medium text-sm text-gray-900 dark:text-white">{log.message}</span>
                              <span className="text-xs text-gray-400 dark:text-gray-500 ml-auto whitespace-nowrap">
                                {formatDate(log.created_at)}
                              </span>
                            </div>
                            {log.details && (
                              <pre className="mt-1.5 text-xs text-gray-600 dark:text-gray-300 bg-white/60 dark:bg-gray-900/40 rounded p-2 overflow-x-auto font-mono">
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
                    );
                  })}

                  {/* Load More */}
                  {!logEventFilter && logs.length < logsTotal && (
                    <div className="p-4 text-center">
                      <button
                        onClick={() => fetchLogs(true)}
                        disabled={logsLoading}
                        className="px-4 py-2 text-sm bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 disabled:opacity-50 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
                      >
                        {logsLoading ? 'Loading...' : `Load More (${logsTotal - logs.length} remaining)`}
                      </button>
                    </div>
                  )}
                  {logEventFilter && logs.length < logsTotal && (
                    <div className="p-4 text-center">
                      <button
                        onClick={() => fetchLogs(true)}
                        disabled={logsLoading}
                        className="px-4 py-2 text-sm bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 disabled:opacity-50 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
                      >
                        {logsLoading ? 'Loading...' : 'Load More to find more matching events'}
                      </button>
                    </div>
                  )}
                </>
              ) : (
                <div className="p-8 text-center text-gray-500 dark:text-gray-400">
                  {logsLoading ? (
                    <RefreshCw className="w-8 h-8 mx-auto text-gray-300 dark:text-gray-600 mb-3 animate-spin" />
                  ) : (
                    <>
                      <FileText className="w-12 h-12 mx-auto text-gray-300 dark:text-gray-600 mb-3" />
                      {logSearch || logEventFilter ? (
                        <>
                          <p>No logs matching your filters.</p>
                          {logEventFilter && logs.length < logsTotal && (
                            <button
                              onClick={() => fetchLogs(true)}
                              className="mt-2 text-sm text-blue-500 hover:underline"
                            >
                              Load more logs to search further
                            </button>
                          )}
                        </>
                      ) : (
                        <>
                          <p>No logs available yet.</p>
                          <p className="text-sm mt-1">Logs will appear as the mirror performs operations.</p>
                        </>
                      )}
                    </>
                  )}
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Delete Confirmation Modal */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl dark:shadow-gray-900/40 max-w-md w-full p-6">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">Delete Mirror?</h3>
            <p className="text-gray-600 dark:text-gray-300 mb-4">
              Are you sure you want to delete &quot;{mirror.name}&quot;? This will stop replication and clean up
              the replication slot and publication on the source database.
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowDeleteConfirm(false)}
                className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg dark:text-gray-300 dark:hover:bg-gray-800"
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
          <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl dark:shadow-gray-900/40 max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col">
            <div className="p-6 border-b dark:border-gray-700 flex items-center justify-between">
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Edit Table Mappings</h3>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">Add or remove tables from this mirror while paused</p>
              </div>
              <button
                onClick={() => setShowTableEditor(false)}
                className="p-2 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg"
              >
                <X className="w-5 h-5 text-gray-500 dark:text-gray-400" />
              </button>
            </div>

            <div className="p-6 overflow-y-auto flex-1">
              {tableEditorError && (
                <div className="mb-4 bg-red-50 border border-red-200 rounded-lg p-3 flex items-center gap-2 dark:bg-red-900/20 dark:border-red-800">
                  <AlertCircle className="w-4 h-4 text-red-500" />
                  <span className="text-sm text-red-700 dark:text-red-400">{tableEditorError}</span>
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
                  className="w-full pl-10 pr-4 py-2 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
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
                  <div key={index} className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 border dark:border-gray-700">
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex-1 grid grid-cols-2 gap-4">
                        <div>
                          <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Source Schema</label>
                          <input
                            type="text"
                            value={mapping.source_schema}
                            onChange={(e) => updateTableMapping(index, 'source_schema', e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg text-sm font-mono dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                            placeholder="public"
                          />
                        </div>
                        <div>
                          <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Source Table</label>
                          <input
                            type="text"
                            value={mapping.source_table}
                            onChange={(e) => updateTableMapping(index, 'source_table', e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg text-sm font-mono dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                            placeholder="table_name"
                          />
                        </div>
                        <div>
                          <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Destination Schema</label>
                          <input
                            type="text"
                            value={mapping.destination_schema}
                            onChange={(e) => updateTableMapping(index, 'destination_schema', e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg text-sm font-mono dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                            placeholder="public"
                          />
                        </div>
                        <div>
                          <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Destination Table</label>
                          <input
                            type="text"
                            value={mapping.destination_table}
                            onChange={(e) => updateTableMapping(index, 'destination_table', e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg text-sm font-mono dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                            placeholder="table_name"
                          />
                        </div>
                      </div>
                      <button
                        onClick={() => removeTableMapping(index)}
                        className="p-2 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg mt-5"
                        title="Remove table"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                ))}

                {tableMappings.length === 0 && (
                  <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                    <Table className="w-12 h-12 mx-auto text-gray-300 dark:text-gray-600 mb-3" />
                    <p>No tables configured</p>
                    <p className="text-sm mt-1">Add tables to replicate</p>
                  </div>
                )}

                {tableMappings.length > 0 && tableEditorSearch && tableMappings.filter(m =>
                  m.source_table.toLowerCase().includes(tableEditorSearch.toLowerCase()) ||
                  m.source_schema.toLowerCase().includes(tableEditorSearch.toLowerCase())
                ).length === 0 && (
                  <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                    <Search className="w-12 h-12 mx-auto text-gray-300 dark:text-gray-600 mb-3" />
                    <p>No tables matching &quot;{tableEditorSearch}&quot;</p>
                  </div>
                )}
              </div>

              {/* Available Tables Section */}
              <div className="mt-6 border-t dark:border-gray-700 pt-4">
                <h4 className="text-sm font-medium text-gray-700 dark:text-gray-200 mb-3 flex items-center gap-2">
                  <Plus className="w-4 h-4" />
                  Add Tables from Source Database
                </h4>
                {loadingAvailableTables ? (
                  <div className="flex items-center justify-center py-4">
                    <RefreshCw className="w-5 h-5 animate-spin text-blue-500" />
                    <span className="ml-2 text-sm text-gray-500 dark:text-gray-400">Loading available tables...</span>
                  </div>
                ) : availableTables.length > 0 ? (
                  <>
                    <div className="max-h-[200px] overflow-y-auto border dark:border-gray-700 rounded-lg divide-y dark:divide-gray-700">
                      {availableTables
                        .filter(t =>
                          tableEditorSearch === '' ||
                          t.table_name.toLowerCase().includes(tableEditorSearch.toLowerCase()) ||
                          t.schema.toLowerCase().includes(tableEditorSearch.toLowerCase())
                        )
                        .map((table) => (
                          <div
                            key={`${table.schema}.${table.table_name}`}
                            className="flex items-center justify-between px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-800"
                          >
                            <span className="font-mono text-sm">
                              {table.schema}.{table.table_name}
                            </span>
                            <button
                              onClick={() => addTableFromAvailable(table.schema, table.table_name)}
                              className="flex items-center gap-1 px-2 py-1 text-xs bg-green-100 text-green-700 rounded hover:bg-green-200 dark:bg-green-900/30 dark:text-green-400 dark:hover:bg-green-900/50"
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
                        <div className="text-center py-4 text-gray-500 dark:text-gray-400 text-sm">
                          No available tables matching search
                        </div>
                      )}
                    </div>
                    <p className="text-xs text-gray-400 mt-2">
                      {availableTables.length} table{availableTables.length !== 1 ? 's' : ''} available to add
                    </p>
                  </>
                ) : (
                  <div className="text-center py-4 text-gray-500 dark:text-gray-400 text-sm border dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <CheckCircle className="w-5 h-5 mx-auto text-green-500 mb-1" />
                    All source tables are already in the mirror
                  </div>
                )}
              </div>
            </div>

            <div className="p-6 border-t dark:border-gray-700 bg-gray-50 dark:bg-gray-800 flex justify-end gap-3">
              <button
                onClick={() => setShowTableEditor(false)}
                className="px-4 py-2 text-gray-700 hover:bg-gray-200 rounded-lg dark:text-gray-300 dark:hover:bg-gray-700"
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
