'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Plus, Trash2, Loader2, Database, ArrowRight } from 'lucide-react';

interface Peer {
  id: number;
  name: string;
  host: string;
  port: number;
  user: string;
  database: string;
}

interface TableMapping {
  source_schema: string;
  source_table: string;
  destination_schema: string;
  destination_table: string;
}

const API_URL = process.env.BUNNY_API_URL || 'http://localhost:8112';

export default function NewMirrorPage() {
  const router = useRouter();
  const [peers, setPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [formData, setFormData] = useState({
    name: '',
    source_peer: '',
    destination_peer: '',
    do_initial_snapshot: true,
    replicate_indexes: true,
    replicate_foreign_keys: true,
    max_batch_size: 1000,
    snapshot_num_rows_per_partition: 250000,
    snapshot_max_parallel_workers: 4,
    snapshot_num_tables_in_parallel: 4,
  });

  const [tableMappings, setTableMappings] = useState<TableMapping[]>([
    { source_schema: 'public', source_table: '', destination_schema: 'public', destination_table: '' }
  ]);

  const fetchPeers = async () => {
    try {
      const res = await fetch(`${API_URL}/v1/peers`);
      if (!res.ok) throw new Error('Failed to fetch peers');
      const data = await res.json();
      setPeers(data || []);
    } catch (err) {
      console.error('Failed to fetch peers:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPeers();
  }, []);

  const addTableMapping = () => {
    setTableMappings([
      ...tableMappings,
      { source_schema: 'public', source_table: '', destination_schema: 'public', destination_table: '' }
    ]);
  };

  const removeTableMapping = (index: number) => {
    setTableMappings(tableMappings.filter((_, i) => i !== index));
  };

  const updateTableMapping = (index: number, field: keyof TableMapping, value: string) => {
    const updated = [...tableMappings];
    updated[index][field] = value;
    // Auto-fill destination if empty
    if (field === 'source_table' && !updated[index].destination_table) {
      updated[index].destination_table = value;
    }
    if (field === 'source_schema' && !updated[index].destination_schema) {
      updated[index].destination_schema = value;
    }
    setTableMappings(updated);
  };

  const createMirror = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validate
    if (!formData.name) {
      setError('Mirror name is required');
      return;
    }
    if (!formData.source_peer || !formData.destination_peer) {
      setError('Source and destination peers are required');
      return;
    }
    if (formData.source_peer === formData.destination_peer) {
      setError('Source and destination peers must be different');
      return;
    }
    const validMappings = tableMappings.filter(m => m.source_table && m.destination_table);
    if (validMappings.length === 0) {
      setError('At least one table mapping is required');
      return;
    }

    setCreating(true);
    try {
      const res = await fetch(`${API_URL}/v1/mirrors`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ...formData,
          table_mappings: validMappings,
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to create mirror');
      }

      router.push('/');
    } catch (err: any) {
      setError(err.message || 'Failed to create mirror');
    } finally {
      setCreating(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  const sourcePeer = peers.find(p => p.name === formData.source_peer);
  const destPeer = peers.find(p => p.name === formData.destination_peer);

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <button
          onClick={() => router.push('/')}
          className="p-2 hover:bg-gray-100 rounded-lg"
        >
          <ArrowLeft className="w-5 h-5" />
        </button>
        <h1 className="text-2xl font-bold text-gray-900">Create Mirror</h1>
      </div>

      {peers.length === 0 ? (
        <div className="bg-white rounded-lg shadow p-8 text-center">
          <Database className="w-12 h-12 mx-auto text-gray-400 mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">No peers configured</h3>
          <p className="text-gray-500 mb-4">You need to add at least two peer connections before creating a mirror.</p>
          <button
            onClick={() => router.push('/peers')}
            className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
          >
            Add Peers
          </button>
        </div>
      ) : peers.length < 2 ? (
        <div className="bg-white rounded-lg shadow p-8 text-center">
          <Database className="w-12 h-12 mx-auto text-gray-400 mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">Need more peers</h3>
          <p className="text-gray-500 mb-4">You need at least two peer connections to create a mirror.</p>
          <button
            onClick={() => router.push('/peers')}
            className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
          >
            Add Another Peer
          </button>
        </div>
      ) : (
        <form onSubmit={createMirror} className="space-y-6">
          {error && (
            <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
              {error}
            </div>
          )}

          {/* Mirror Name */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-lg font-semibold mb-4">Mirror Configuration</h2>
            <div>
              <label className="block text-sm font-medium text-gray-700">Mirror Name</label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                className="mt-1 block w-full max-w-md rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2"
                placeholder="my-replication-mirror"
                required
              />
            </div>
          </div>

          {/* Peer Selection */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-lg font-semibold mb-4">Source & Destination</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 items-center">
              <div>
                <label className="block text-sm font-medium text-gray-700">Source Peer</label>
                <select
                  value={formData.source_peer}
                  onChange={(e) => setFormData({ ...formData, source_peer: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2"
                  required
                >
                  <option value="">Select source...</option>
                  {peers.map(peer => (
                    <option key={peer.id} value={peer.name} disabled={peer.name === formData.destination_peer}>
                      {peer.name} ({peer.database})
                    </option>
                  ))}
                </select>
                {sourcePeer && (
                  <p className="text-xs text-gray-500 mt-1">
                    {sourcePeer.user}@{sourcePeer.host}:{sourcePeer.port}
                  </p>
                )}
              </div>

              <div className="flex justify-center">
                <ArrowRight className="w-8 h-8 text-gray-400" />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700">Destination Peer</label>
                <select
                  value={formData.destination_peer}
                  onChange={(e) => setFormData({ ...formData, destination_peer: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2"
                  required
                >
                  <option value="">Select destination...</option>
                  {peers.map(peer => (
                    <option key={peer.id} value={peer.name} disabled={peer.name === formData.source_peer}>
                      {peer.name} ({peer.database})
                    </option>
                  ))}
                </select>
                {destPeer && (
                  <p className="text-xs text-gray-500 mt-1">
                    {destPeer.user}@{destPeer.host}:{destPeer.port}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Table Mappings */}
          <div className="bg-white rounded-lg shadow p-6">
            <div className="flex justify-between items-center mb-4">
              <h2 className="text-lg font-semibold">Table Mappings</h2>
              <button
                type="button"
                onClick={addTableMapping}
                className="flex items-center gap-1 px-3 py-1.5 bg-blue-100 text-blue-700 rounded hover:bg-blue-200"
              >
                <Plus className="w-4 h-4" />
                Add Table
              </button>
            </div>

            <div className="space-y-4">
              {tableMappings.map((mapping, index) => (
                <div key={index} className="grid grid-cols-1 md:grid-cols-9 gap-2 items-end p-4 bg-gray-50 rounded-lg">
                  <div className="md:col-span-2">
                    <label className="block text-xs font-medium text-gray-500">Source Schema</label>
                    <input
                      type="text"
                      value={mapping.source_schema}
                      onChange={(e) => updateTableMapping(index, 'source_schema', e.target.value)}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 text-sm"
                      placeholder="public"
                    />
                  </div>
                  <div className="md:col-span-2">
                    <label className="block text-xs font-medium text-gray-500">Source Table</label>
                    <input
                      type="text"
                      value={mapping.source_table}
                      onChange={(e) => updateTableMapping(index, 'source_table', e.target.value)}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 text-sm"
                      placeholder="users"
                      required
                    />
                  </div>
                  <div className="flex justify-center items-center">
                    <ArrowRight className="w-5 h-5 text-gray-400" />
                  </div>
                  <div className="md:col-span-2">
                    <label className="block text-xs font-medium text-gray-500">Dest Schema</label>
                    <input
                      type="text"
                      value={mapping.destination_schema}
                      onChange={(e) => updateTableMapping(index, 'destination_schema', e.target.value)}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 text-sm"
                      placeholder="public"
                    />
                  </div>
                  <div className="md:col-span-2">
                    <label className="block text-xs font-medium text-gray-500">Dest Table</label>
                    <input
                      type="text"
                      value={mapping.destination_table}
                      onChange={(e) => updateTableMapping(index, 'destination_table', e.target.value)}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 text-sm"
                      placeholder="users"
                      required
                    />
                  </div>
                  {tableMappings.length > 1 && (
                    <button
                      type="button"
                      onClick={() => removeTableMapping(index)}
                      className="p-2 text-red-500 hover:bg-red-50 rounded"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* Options */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-lg font-semibold mb-4">Replication Options</h2>
            <div className="space-y-4">
              <label className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.do_initial_snapshot}
                  onChange={(e) => setFormData({ ...formData, do_initial_snapshot: e.target.checked })}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                />
                <div>
                  <span className="font-medium">Initial Snapshot</span>
                  <p className="text-sm text-gray-500">Copy existing data before starting CDC</p>
                </div>
              </label>

              <label className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.replicate_indexes}
                  onChange={(e) => setFormData({ ...formData, replicate_indexes: e.target.checked })}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                />
                <div>
                  <span className="font-medium">Replicate Indexes</span>
                  <p className="text-sm text-gray-500">Create indexes on destination tables</p>
                </div>
              </label>

              <label className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.replicate_foreign_keys}
                  onChange={(e) => setFormData({ ...formData, replicate_foreign_keys: e.target.checked })}
                  className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                />
                <div>
                  <span className="font-medium">Replicate Foreign Keys</span>
                  <p className="text-sm text-gray-500">Recreate FK constraints using defer strategy</p>
                </div>
              </label>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-6">
              <div>
                <label className="block text-sm font-medium text-gray-700">Batch Size</label>
                <input
                  type="number"
                  value={formData.max_batch_size}
                  onChange={(e) => setFormData({ ...formData, max_batch_size: parseInt(e.target.value) })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Rows/Partition</label>
                <input
                  type="number"
                  value={formData.snapshot_num_rows_per_partition}
                  onChange={(e) => setFormData({ ...formData, snapshot_num_rows_per_partition: parseInt(e.target.value) })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Parallel Workers</label>
                <input
                  type="number"
                  value={formData.snapshot_max_parallel_workers}
                  onChange={(e) => setFormData({ ...formData, snapshot_max_parallel_workers: parseInt(e.target.value) })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Tables in Parallel</label>
                <input
                  type="number"
                  value={formData.snapshot_num_tables_in_parallel}
                  onChange={(e) => setFormData({ ...formData, snapshot_num_tables_in_parallel: parseInt(e.target.value) })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2"
                />
              </div>
            </div>
          </div>

          {/* Submit */}
          <div className="flex justify-end gap-4">
            <button
              type="button"
              onClick={() => router.push('/')}
              className="px-6 py-2 text-gray-700 hover:bg-gray-100 rounded-lg"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={creating}
              className="flex items-center gap-2 px-6 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 disabled:opacity-50"
            >
              {creating ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Creating...
                </>
              ) : (
                'Create Mirror'
              )}
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
