'use client';

import { useState, useEffect, useMemo } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Plus, Trash2, Loader2, Database, ArrowRight, Search, CheckSquare, Square, RefreshCw } from 'lucide-react';
import { authFetch } from '../../../lib/auth';

interface Peer {
  id: number;
  name: string;
  host: string;
  port: number;
  user: string;
  database: string;
}

interface TableInfo {
  schema: string;
  table_name: string;
}

interface TableMapping {
  source_schema: string;
  source_table: string;
  destination_schema: string;
  destination_table: string;
}

export default function NewMirrorPage() {
  const router = useRouter();
  const [peers, setPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Source tables
  const [sourceTables, setSourceTables] = useState<TableInfo[]>([]);
  const [loadingTables, setLoadingTables] = useState(false);
  const [tableSearch, setTableSearch] = useState('');
  const [selectedTables, setSelectedTables] = useState<Set<string>>(new Set());

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

  const [tableMappings, setTableMappings] = useState<TableMapping[]>([]);

  const fetchPeers = async () => {
    try {
      const res = await authFetch('/v1/peers');
      if (!res.ok) throw new Error('Failed to fetch peers');
      const data = await res.json();
      setPeers(data || []);
    } catch (err) {
      console.error('Failed to fetch peers:', err);
    } finally {
      setLoading(false);
    }
  };

  const fetchSourceTables = async (peerName: string) => {
    if (!peerName) {
      setSourceTables([]);
      return;
    }

    setLoadingTables(true);
    try {
      const res = await authFetch(`/v1/peers/${peerName}/tables`);
      if (!res.ok) throw new Error('Failed to fetch tables');
      const data = await res.json();
      setSourceTables(data || []);
      setSelectedTables(new Set());
      setTableMappings([]);
    } catch (err) {
      console.error('Failed to fetch tables:', err);
      setSourceTables([]);
    } finally {
      setLoadingTables(false);
    }
  };

  useEffect(() => {
    fetchPeers();
  }, []);

  useEffect(() => {
    if (formData.source_peer) {
      fetchSourceTables(formData.source_peer);
    }
  }, [formData.source_peer]);

  // Filter tables based on search
  const filteredTables = useMemo(() => {
    if (!tableSearch) return sourceTables;
    const search = tableSearch.toLowerCase();
    return sourceTables.filter(t =>
      t.table_name.toLowerCase().includes(search) ||
      t.schema.toLowerCase().includes(search)
    );
  }, [sourceTables, tableSearch]);

  // Group tables by schema
  const tablesBySchema = useMemo(() => {
    const grouped: Record<string, TableInfo[]> = {};
    filteredTables.forEach(t => {
      if (!grouped[t.schema]) grouped[t.schema] = [];
      grouped[t.schema].push(t);
    });
    return grouped;
  }, [filteredTables]);

  const getTableKey = (t: TableInfo) => `${t.schema}.${t.table_name}`;

  const toggleTable = (table: TableInfo) => {
    const key = getTableKey(table);
    const newSelected = new Set(selectedTables);
    if (newSelected.has(key)) {
      newSelected.delete(key);
    } else {
      newSelected.add(key);
    }
    setSelectedTables(newSelected);
    updateMappingsFromSelection(newSelected);
  };

  const toggleAllFiltered = () => {
    const filteredKeys = new Set(filteredTables.map(getTableKey));
    const allSelected = filteredTables.every(t => selectedTables.has(getTableKey(t)));

    const newSelected = new Set(selectedTables);
    if (allSelected) {
      // Deselect all filtered
      filteredKeys.forEach(k => newSelected.delete(k));
    } else {
      // Select all filtered
      filteredKeys.forEach(k => newSelected.add(k));
    }
    setSelectedTables(newSelected);
    updateMappingsFromSelection(newSelected);
  };

  const toggleSchema = (schema: string) => {
    const schemaTables = tablesBySchema[schema] || [];
    const schemaKeys = schemaTables.map(getTableKey);
    const allSelected = schemaKeys.every(k => selectedTables.has(k));

    const newSelected = new Set(selectedTables);
    if (allSelected) {
      schemaKeys.forEach(k => newSelected.delete(k));
    } else {
      schemaKeys.forEach(k => newSelected.add(k));
    }
    setSelectedTables(newSelected);
    updateMappingsFromSelection(newSelected);
  };

  const updateMappingsFromSelection = (selected: Set<string>) => {
    const mappings: TableMapping[] = [];
    selected.forEach(key => {
      const [schema, table] = key.split('.');
      mappings.push({
        source_schema: schema,
        source_table: table,
        destination_schema: schema,
        destination_table: table,
      });
    });
    // Sort by schema then table name
    mappings.sort((a, b) => {
      if (a.source_schema !== b.source_schema) {
        return a.source_schema.localeCompare(b.source_schema);
      }
      return a.source_table.localeCompare(b.source_table);
    });
    setTableMappings(mappings);
  };

  const updateMapping = (index: number, field: keyof TableMapping, value: string) => {
    const updated = [...tableMappings];
    updated[index][field] = value;
    setTableMappings(updated);
  };

  const removeMapping = (index: number) => {
    const mapping = tableMappings[index];
    const key = `${mapping.source_schema}.${mapping.source_table}`;
    const newSelected = new Set(selectedTables);
    newSelected.delete(key);
    setSelectedTables(newSelected);
    setTableMappings(tableMappings.filter((_, i) => i !== index));
  };

  const createMirror = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

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
    if (tableMappings.length === 0) {
      setError('At least one table must be selected');
      return;
    }

    setCreating(true);
    try {
      const res = await authFetch('/v1/mirrors', {
        method: 'POST',
        body: JSON.stringify({
          ...formData,
          table_mappings: tableMappings,
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
  const allFilteredSelected = filteredTables.length > 0 && filteredTables.every(t => selectedTables.has(getTableKey(t)));

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <button
          onClick={() => router.push('/')}
          className="p-2 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg"
        >
          <ArrowLeft className="w-5 h-5" />
        </button>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Create Mirror</h1>
      </div>

      {peers.length === 0 ? (
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-8 text-center">
          <Database className="w-12 h-12 mx-auto text-gray-400 dark:text-gray-500 mb-4" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No peers configured</h3>
          <p className="text-gray-500 dark:text-gray-400 mb-4">You need to add at least two peer connections before creating a mirror.</p>
          <button
            onClick={() => router.push('/peers')}
            className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
          >
            Add Peers
          </button>
        </div>
      ) : peers.length < 2 ? (
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-8 text-center">
          <Database className="w-12 h-12 mx-auto text-gray-400 dark:text-gray-500 mb-4" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">Need more peers</h3>
          <p className="text-gray-500 dark:text-gray-400 mb-4">You need at least two peer connections to create a mirror.</p>
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
            <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 dark:bg-red-900/20 dark:border-red-800 dark:text-red-400">
              {error}
            </div>
          )}

          {/* Mirror Name */}
          <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-6">
            <h2 className="text-lg font-semibold mb-4 dark:text-white">Mirror Configuration</h2>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Mirror Name</label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                className="mt-1 block w-full max-w-md rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                placeholder="my-replication-mirror"
                required
              />
            </div>
          </div>

          {/* Peer Selection */}
          <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-6">
            <h2 className="text-lg font-semibold mb-4 dark:text-white">Source & Destination</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 items-center">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Source Peer</label>
                <select
                  value={formData.source_peer}
                  onChange={(e) => setFormData({ ...formData, source_peer: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
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
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    {sourcePeer.user}@{sourcePeer.host}:{sourcePeer.port}
                  </p>
                )}
              </div>

              <div className="flex justify-center">
                <ArrowRight className="w-8 h-8 text-gray-400 dark:text-gray-500" />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Destination Peer</label>
                <select
                  value={formData.destination_peer}
                  onChange={(e) => setFormData({ ...formData, destination_peer: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
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
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    {destPeer.user}@{destPeer.host}:{destPeer.port}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Table Selection */}
          <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-6">
            <div className="flex justify-between items-center mb-4">
              <h2 className="text-lg font-semibold dark:text-white">Select Tables</h2>
              {formData.source_peer && (
                <button
                  type="button"
                  onClick={() => fetchSourceTables(formData.source_peer)}
                  className="flex items-center gap-1 px-3 py-1.5 text-gray-600 hover:bg-gray-100 rounded dark:text-gray-300 dark:hover:bg-gray-800"
                >
                  <RefreshCw className={`w-4 h-4 ${loadingTables ? 'animate-spin' : ''}`} />
                  Refresh
                </button>
              )}
            </div>

            {!formData.source_peer ? (
              <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                Select a source peer to load available tables
              </div>
            ) : loadingTables ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="w-6 h-6 animate-spin text-blue-500" />
                <span className="ml-2 text-gray-500 dark:text-gray-400">Loading tables...</span>
              </div>
            ) : sourceTables.length === 0 ? (
              <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                No tables found in the source database
              </div>
            ) : (
              <>
                {/* Search and Select All */}
                <div className="flex gap-4 mb-4">
                  <div className="flex-1 relative">
                    <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                    <input
                      type="text"
                      value={tableSearch}
                      onChange={(e) => setTableSearch(e.target.value)}
                      placeholder="Search tables..."
                      className="w-full pl-10 pr-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
                    />
                  </div>
                  <button
                    type="button"
                    onClick={toggleAllFiltered}
                    className="flex items-center gap-2 px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
                  >
                    {allFilteredSelected ? (
                      <CheckSquare className="w-4 h-4" />
                    ) : (
                      <Square className="w-4 h-4" />
                    )}
                    {allFilteredSelected ? 'Deselect All' : 'Select All'}
                  </button>
                </div>

                {/* Selected count */}
                <div className="mb-4 text-sm text-gray-600 dark:text-gray-300">
                  {selectedTables.size} of {sourceTables.length} tables selected
                </div>

                {/* Tables grouped by schema */}
                <div className="max-h-64 overflow-y-auto border dark:border-gray-700 rounded-lg">
                  {Object.entries(tablesBySchema).map(([schema, tables]) => {
                    const schemaKeys = tables.map(getTableKey);
                    const allSchemaSelected = schemaKeys.every(k => selectedTables.has(k));
                    const someSchemaSelected = schemaKeys.some(k => selectedTables.has(k));

                    return (
                      <div key={schema} className="border-b dark:border-gray-700 last:border-b-0">
                        {/* Schema header */}
                        <div
                          className="flex items-center gap-2 px-4 py-2 bg-gray-50 dark:bg-gray-800 cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-700"
                          onClick={() => toggleSchema(schema)}
                        >
                          {allSchemaSelected ? (
                            <CheckSquare className="w-4 h-4 text-blue-500" />
                          ) : someSchemaSelected ? (
                            <div className="w-4 h-4 border-2 border-blue-500 bg-blue-100 dark:bg-blue-900/30 rounded" />
                          ) : (
                            <Square className="w-4 h-4 text-gray-400" />
                          )}
                          <span className="font-medium text-gray-700 dark:text-gray-200">{schema}</span>
                          <span className="text-xs text-gray-500 dark:text-gray-400">({tables.length} tables)</span>
                        </div>
                        {/* Tables */}
                        <div className="divide-y dark:divide-gray-700">
                          {tables.map(table => {
                            const isSelected = selectedTables.has(getTableKey(table));
                            return (
                              <div
                                key={getTableKey(table)}
                                className={`flex items-center gap-2 px-4 py-2 pl-8 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 ${
                                  isSelected ? 'bg-blue-50 dark:bg-blue-900/20' : ''
                                }`}
                                onClick={() => toggleTable(table)}
                              >
                                {isSelected ? (
                                  <CheckSquare className="w-4 h-4 text-blue-500" />
                                ) : (
                                  <Square className="w-4 h-4 text-gray-400" />
                                )}
                                <span className={isSelected ? 'text-blue-700 dark:text-blue-400' : 'text-gray-700 dark:text-gray-200'}>
                                  {table.table_name}
                                </span>
                              </div>
                            );
                          })}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </>
            )}
          </div>

          {/* Selected Table Mappings */}
          {tableMappings.length > 0 && (
            <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-6">
              <h2 className="text-lg font-semibold mb-4 dark:text-white">
                Table Mappings ({tableMappings.length})
              </h2>
              <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                Customize destination schema/table names if needed
              </p>

              <div className="space-y-2 max-h-64 overflow-y-auto">
                {tableMappings.map((mapping, index) => (
                  <div key={index} className="grid grid-cols-12 gap-2 items-center p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                    <div className="col-span-5 flex items-center gap-2">
                      <span className="text-xs text-gray-500 dark:text-gray-400 w-16">Source:</span>
                      <span className="font-mono text-sm">
                        {mapping.source_schema}.{mapping.source_table}
                      </span>
                    </div>
                    <div className="col-span-1 flex justify-center">
                      <ArrowRight className="w-4 h-4 text-gray-400" />
                    </div>
                    <div className="col-span-2">
                      <input
                        type="text"
                        value={mapping.destination_schema}
                        onChange={(e) => updateMapping(index, 'destination_schema', e.target.value)}
                        className="w-full text-sm rounded border-gray-300 dark:border-gray-600 border p-1 dark:bg-gray-700 dark:text-white"
                        placeholder="schema"
                      />
                    </div>
                    <div className="col-span-3">
                      <input
                        type="text"
                        value={mapping.destination_table}
                        onChange={(e) => updateMapping(index, 'destination_table', e.target.value)}
                        className="w-full text-sm rounded border-gray-300 dark:border-gray-600 border p-1 dark:bg-gray-700 dark:text-white"
                        placeholder="table"
                      />
                    </div>
                    <div className="col-span-1 flex justify-end">
                      <button
                        type="button"
                        onClick={() => removeMapping(index)}
                        className="p-1 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Options */}
          <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-6">
            <h2 className="text-lg font-semibold mb-4 dark:text-white">Replication Options</h2>
            <div className="space-y-4">
              <label className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.do_initial_snapshot}
                  onChange={(e) => setFormData({ ...formData, do_initial_snapshot: e.target.checked })}
                  className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                />
                <div>
                  <span className="font-medium dark:text-white">Initial Snapshot</span>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Copy existing data before starting CDC</p>
                </div>
              </label>

              <label className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.replicate_indexes}
                  onChange={(e) => setFormData({ ...formData, replicate_indexes: e.target.checked })}
                  className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                />
                <div>
                  <span className="font-medium dark:text-white">Replicate Indexes</span>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Create indexes on destination tables</p>
                </div>
              </label>

              <label className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.replicate_foreign_keys}
                  onChange={(e) => setFormData({ ...formData, replicate_foreign_keys: e.target.checked })}
                  className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                />
                <div>
                  <span className="font-medium dark:text-white">Replicate Foreign Keys</span>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Recreate FK constraints using defer strategy</p>
                </div>
              </label>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Batch Size</label>
                <input
                  type="number"
                  value={formData.max_batch_size}
                  onChange={(e) => setFormData({ ...formData, max_batch_size: parseInt(e.target.value) })}
                  className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Rows/Partition</label>
                <input
                  type="number"
                  value={formData.snapshot_num_rows_per_partition}
                  onChange={(e) => setFormData({ ...formData, snapshot_num_rows_per_partition: parseInt(e.target.value) })}
                  className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Parallel Workers</label>
                <input
                  type="number"
                  value={formData.snapshot_max_parallel_workers}
                  onChange={(e) => setFormData({ ...formData, snapshot_max_parallel_workers: parseInt(e.target.value) })}
                  className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Tables in Parallel</label>
                <input
                  type="number"
                  value={formData.snapshot_num_tables_in_parallel}
                  onChange={(e) => setFormData({ ...formData, snapshot_num_tables_in_parallel: parseInt(e.target.value) })}
                  className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                />
              </div>
            </div>
          </div>

          {/* Submit */}
          <div className="flex justify-end gap-4">
            <button
              type="button"
              onClick={() => router.push('/')}
              className="px-6 py-2 text-gray-700 hover:bg-gray-100 rounded-lg dark:text-gray-300 dark:hover:bg-gray-800"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={creating || tableMappings.length === 0}
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
