'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Play, Pause, RefreshCw, Trash2, RotateCcw, Database, AlertCircle, Plus } from 'lucide-react';
import { getStatusColor, getStatusIcon } from '../../lib/status';
import { authFetch, isAdmin } from '../../lib/auth';

interface Mirror {
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

interface TableStatus {
  table_name: string;
  status: string;
  rows_synced: number;
  last_synced_at?: string;
}

export default function Home() {
  const router = useRouter();
  const [mirrors, setMirrors] = useState<Mirror[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const admin = isAdmin();

  const fetchMirrors = async () => {
    try {
      const res = await authFetch('/v1/mirrors');
      if (!res.ok) throw new Error('Failed to fetch mirrors');
      const data = await res.json();
      setMirrors(data || []);
      setError(null);
    } catch (err) {
      setError('Failed to connect to BunnyDB API');
    } finally {
      setLoading(false);
    }
  };

  const performAction = async (name: string, action: string) => {
    try {
      const res = await authFetch(`/v1/mirrors/${name}/${action}`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`Failed to ${action} mirror`);
      fetchMirrors();
    } catch (err) {
      console.error(`Failed to ${action}:`, err);
    }
  };

  useEffect(() => {
    fetchMirrors();
    const interval = setInterval(fetchMirrors, 5000);
    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="w-8 h-8 animate-spin text-bunny-500" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Mirrors</h1>
        <div className="flex items-center gap-2">
          <button
            onClick={fetchMirrors}
            className="flex items-center gap-2 px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
          {admin && (
            <button
              onClick={() => router.push('/mirrors/new')}
              className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
            >
              <Plus className="w-4 h-4" />
              Create Mirror
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 flex items-center gap-2 dark:bg-red-900/20 dark:border-red-800">
          <AlertCircle className="w-5 h-5 text-red-500" />
          <span className="text-red-700 dark:text-red-400">{error}</span>
        </div>
      )}

      {mirrors.length === 0 ? (
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-8 text-center">
          <Database className="w-12 h-12 mx-auto text-gray-400 dark:text-gray-500 mb-4" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No mirrors yet</h3>
          <p className="text-gray-500 dark:text-gray-400 mb-4">Create your first mirror to start replicating data.</p>
          {admin && (
            <button
              onClick={() => router.push('/mirrors/new')}
              className="inline-flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
            >
              <Plus className="w-4 h-4" />
              Create Mirror
            </button>
          )}
        </div>
      ) : (
        <div className="grid gap-4">
          {mirrors.map((mirror) => (
            <div
              key={mirror.name}
              className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 hover:shadow-md dark:hover:shadow-gray-900/40 transition-shadow cursor-pointer"
              onClick={() => router.push(`/mirrors/${encodeURIComponent(mirror.name)}`)}
            >
              <div className="p-6">
                <div className="flex justify-between items-start">
                  <div className="flex items-center gap-3">
                    {getStatusIcon(mirror.status)}
                    <div>
                      <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{mirror.name}</h3>
                      <p className="text-sm text-gray-500 dark:text-gray-400">
                        Slot: {mirror.slot_name || 'N/A'} | Publication: {mirror.publication_name || 'N/A'}
                      </p>
                    </div>
                  </div>
                  <span className={`px-3 py-1 rounded-full text-sm font-medium ${getStatusColor(mirror.status)}`}>
                    {mirror.status}
                  </span>
                </div>

                {mirror.error_message && (
                  <div className="mt-4 bg-red-50 border border-red-200 rounded p-3 dark:bg-red-900/20 dark:border-red-800">
                    <p className="text-sm text-red-700 dark:text-red-400">{mirror.error_message}</p>
                    <p className="text-xs text-red-500 dark:text-red-400 mt-1">Error count: {mirror.error_count}</p>
                  </div>
                )}

                {admin && (
                  <div className="mt-4 flex gap-2">
                    {!['PAUSED', 'PAUSING', 'TERMINATED', 'TERMINATING', 'FAILED'].includes(mirror.status?.toUpperCase()) && (
                      <button
                        onClick={(e) => { e.stopPropagation(); performAction(mirror.name, 'pause'); }}
                        className="flex items-center gap-1 px-3 py-1.5 bg-yellow-100 text-yellow-700 rounded hover:bg-yellow-200 dark:bg-yellow-900/30 dark:text-yellow-400 dark:hover:bg-yellow-900/50"
                      >
                        <Pause className="w-4 h-4" />
                        Pause
                      </button>
                    )}
                    {mirror.status?.toUpperCase() === 'PAUSED' && (
                      <button
                        onClick={(e) => { e.stopPropagation(); performAction(mirror.name, 'resume'); }}
                        className="flex items-center gap-1 px-3 py-1.5 bg-green-100 text-green-700 rounded hover:bg-green-200 dark:bg-green-900/30 dark:text-green-400 dark:hover:bg-green-900/50"
                      >
                        <Play className="w-4 h-4" />
                        Resume
                      </button>
                    )}
                    <button
                      onClick={(e) => { e.stopPropagation(); performAction(mirror.name, 'retry'); }}
                      className="flex items-center gap-1 px-3 py-1.5 bg-blue-100 text-blue-700 rounded hover:bg-blue-200 dark:bg-blue-900/30 dark:text-blue-400 dark:hover:bg-blue-900/50"
                    >
                      <RotateCcw className="w-4 h-4" />
                      Retry Now
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); performAction(mirror.name, 'resync'); }}
                      className="flex items-center gap-1 px-3 py-1.5 bg-purple-100 text-purple-700 rounded hover:bg-purple-200 dark:bg-purple-900/30 dark:text-purple-400 dark:hover:bg-purple-900/50"
                    >
                      <RefreshCw className="w-4 h-4" />
                      Resync
                    </button>
                  </div>
                )}

                <div className="mt-4 grid grid-cols-3 gap-4 text-sm">
                  <div>
                    <span className="text-gray-500 dark:text-gray-400">Last LSN:</span>
                    <span className="ml-2 font-mono">{mirror.last_lsn || 0}</span>
                  </div>
                  <div>
                    <span className="text-gray-500 dark:text-gray-400">Batch ID:</span>
                    <span className="ml-2 font-mono">{mirror.last_sync_batch_id || 0}</span>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
