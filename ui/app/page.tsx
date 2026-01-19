'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Play, Pause, RefreshCw, Trash2, RotateCcw, Database, AlertCircle, CheckCircle, Clock, Plus } from 'lucide-react';

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

const API_URL = process.env.BUNNY_API_URL || 'http://localhost:8112';

export default function Home() {
  const router = useRouter();
  const [mirrors, setMirrors] = useState<Mirror[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedMirror, setSelectedMirror] = useState<Mirror | null>(null);

  const fetchMirrors = async () => {
    try {
      const res = await fetch(`${API_URL}/v1/mirrors`);
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

  const fetchMirrorDetails = async (name: string) => {
    try {
      const res = await fetch(`${API_URL}/v1/mirrors/${name}`);
      if (!res.ok) throw new Error('Failed to fetch mirror details');
      const data = await res.json();
      setSelectedMirror(data);
    } catch (err) {
      console.error('Failed to fetch mirror details:', err);
    }
  };

  const performAction = async (name: string, action: string) => {
    try {
      const res = await fetch(`${API_URL}/v1/mirrors/${name}/${action}`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error(`Failed to ${action} mirror`);
      fetchMirrors();
      if (selectedMirror?.name === name) {
        fetchMirrorDetails(name);
      }
    } catch (err) {
      console.error(`Failed to ${action}:`, err);
    }
  };

  useEffect(() => {
    fetchMirrors();
    const interval = setInterval(fetchMirrors, 5000);
    return () => clearInterval(interval);
  }, []);

  const getStatusColor = (status: string) => {
    switch (status.toUpperCase()) {
      case 'RUNNING':
        return 'bg-green-100 text-green-800';
      case 'PAUSED':
        return 'bg-yellow-100 text-yellow-800';
      case 'FAILED':
      case 'ERROR':
        return 'bg-red-100 text-red-800';
      case 'SNAPSHOT':
      case 'RESYNCING':
        return 'bg-blue-100 text-blue-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status.toUpperCase()) {
      case 'RUNNING':
        return <CheckCircle className="w-4 h-4 text-green-500" />;
      case 'PAUSED':
        return <Pause className="w-4 h-4 text-yellow-500" />;
      case 'FAILED':
      case 'ERROR':
        return <AlertCircle className="w-4 h-4 text-red-500" />;
      case 'SNAPSHOT':
      case 'RESYNCING':
        return <RefreshCw className="w-4 h-4 text-blue-500 animate-spin" />;
      default:
        return <Clock className="w-4 h-4 text-gray-500" />;
    }
  };

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
        <h1 className="text-2xl font-bold text-gray-900">Mirrors</h1>
        <div className="flex items-center gap-2">
          <button
            onClick={fetchMirrors}
            className="flex items-center gap-2 px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
          <button
            onClick={() => router.push('/mirrors/new')}
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
          >
            <Plus className="w-4 h-4" />
            Create Mirror
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 flex items-center gap-2">
          <AlertCircle className="w-5 h-5 text-red-500" />
          <span className="text-red-700">{error}</span>
        </div>
      )}

      {mirrors.length === 0 ? (
        <div className="bg-white rounded-lg shadow p-8 text-center">
          <Database className="w-12 h-12 mx-auto text-gray-400 mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">No mirrors yet</h3>
          <p className="text-gray-500 mb-4">Create your first mirror to start replicating data.</p>
          <button
            onClick={() => router.push('/mirrors/new')}
            className="inline-flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
          >
            <Plus className="w-4 h-4" />
            Create Mirror
          </button>
        </div>
      ) : (
        <div className="grid gap-4">
          {mirrors.map((mirror) => (
            <div
              key={mirror.name}
              className="bg-white rounded-lg shadow hover:shadow-md transition-shadow cursor-pointer"
              onClick={() => fetchMirrorDetails(mirror.name)}
            >
              <div className="p-6">
                <div className="flex justify-between items-start">
                  <div className="flex items-center gap-3">
                    {getStatusIcon(mirror.status)}
                    <div>
                      <h3 className="text-lg font-semibold text-gray-900">{mirror.name}</h3>
                      <p className="text-sm text-gray-500">
                        Slot: {mirror.slot_name || 'N/A'} | Publication: {mirror.publication_name || 'N/A'}
                      </p>
                    </div>
                  </div>
                  <span className={`px-3 py-1 rounded-full text-sm font-medium ${getStatusColor(mirror.status)}`}>
                    {mirror.status}
                  </span>
                </div>

                {mirror.error_message && (
                  <div className="mt-4 bg-red-50 border border-red-200 rounded p-3">
                    <p className="text-sm text-red-700">{mirror.error_message}</p>
                    <p className="text-xs text-red-500 mt-1">Error count: {mirror.error_count}</p>
                  </div>
                )}

                <div className="mt-4 flex gap-2">
                  {mirror.status === 'RUNNING' && (
                    <button
                      onClick={(e) => { e.stopPropagation(); performAction(mirror.name, 'pause'); }}
                      className="flex items-center gap-1 px-3 py-1.5 bg-yellow-100 text-yellow-700 rounded hover:bg-yellow-200"
                    >
                      <Pause className="w-4 h-4" />
                      Pause
                    </button>
                  )}
                  {mirror.status === 'PAUSED' && (
                    <button
                      onClick={(e) => { e.stopPropagation(); performAction(mirror.name, 'resume'); }}
                      className="flex items-center gap-1 px-3 py-1.5 bg-green-100 text-green-700 rounded hover:bg-green-200"
                    >
                      <Play className="w-4 h-4" />
                      Resume
                    </button>
                  )}
                  <button
                    onClick={(e) => { e.stopPropagation(); performAction(mirror.name, 'retry'); }}
                    className="flex items-center gap-1 px-3 py-1.5 bg-blue-100 text-blue-700 rounded hover:bg-blue-200"
                  >
                    <RotateCcw className="w-4 h-4" />
                    Retry Now
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); performAction(mirror.name, 'resync'); }}
                    className="flex items-center gap-1 px-3 py-1.5 bg-purple-100 text-purple-700 rounded hover:bg-purple-200"
                  >
                    <RefreshCw className="w-4 h-4" />
                    Resync
                  </button>
                </div>

                <div className="mt-4 grid grid-cols-3 gap-4 text-sm">
                  <div>
                    <span className="text-gray-500">Last LSN:</span>
                    <span className="ml-2 font-mono">{mirror.last_lsn || 0}</span>
                  </div>
                  <div>
                    <span className="text-gray-500">Batch ID:</span>
                    <span className="ml-2 font-mono">{mirror.last_sync_batch_id || 0}</span>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Mirror Details Modal */}
      {selectedMirror && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[80vh] overflow-auto">
            <div className="p-6 border-b">
              <div className="flex justify-between items-center">
                <h2 className="text-xl font-bold">{selectedMirror.name}</h2>
                <button
                  onClick={() => setSelectedMirror(null)}
                  className="text-gray-400 hover:text-gray-600"
                >
                  ✕
                </button>
              </div>
            </div>
            <div className="p-6 space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-gray-500">Status</label>
                  <p className={`inline-block px-2 py-1 rounded text-sm ${getStatusColor(selectedMirror.status)}`}>
                    {selectedMirror.status}
                  </p>
                </div>
                <div>
                  <label className="text-sm text-gray-500">Last LSN</label>
                  <p className="font-mono">{selectedMirror.last_lsn}</p>
                </div>
                <div>
                  <label className="text-sm text-gray-500">Slot</label>
                  <p className="font-mono text-sm">{selectedMirror.slot_name}</p>
                </div>
                <div>
                  <label className="text-sm text-gray-500">Publication</label>
                  <p className="font-mono text-sm">{selectedMirror.publication_name}</p>
                </div>
              </div>

              {selectedMirror.tables && selectedMirror.tables.length > 0 && (
                <div>
                  <h3 className="font-semibold mb-2">Tables</h3>
                  <div className="border rounded divide-y">
                    {selectedMirror.tables.map((table) => (
                      <div key={table.table_name} className="p-3 flex justify-between items-center">
                        <div>
                          <span className="font-mono text-sm">{table.table_name}</span>
                          <p className="text-xs text-gray-500">{table.rows_synced} rows synced</p>
                        </div>
                        <div className="flex items-center gap-2">
                          <span className={`px-2 py-1 rounded text-xs ${getStatusColor(table.status)}`}>
                            {table.status}
                          </span>
                          <button
                            onClick={() => performAction(selectedMirror.name, `resync/${table.table_name}`)}
                            className="p-1 text-gray-400 hover:text-blue-500"
                            title="Resync this table"
                          >
                            <RefreshCw className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
