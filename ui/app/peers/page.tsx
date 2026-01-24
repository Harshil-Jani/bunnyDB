'use client';

import { useState, useEffect } from 'react';
import { Plus, Trash2, TestTube, Database, CheckCircle, XCircle, Loader2, Pencil } from 'lucide-react';
import { authFetch, isAdmin } from '../../lib/auth';

interface Peer {
  id: number;
  name: string;
  host: string;
  port: number;
  user: string;
  database: string;
  ssl_mode: string;
}

interface TestResult {
  success: boolean;
  version?: string;
  error?: string;
}

const emptyFormData = {
  name: '',
  host: '',
  port: 5432,
  user: 'postgres',
  password: '',
  database: '',
  ssl_mode: 'prefer',
};

export default function PeersPage() {
  const [peers, setPeers] = useState<Peer[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editingPeer, setEditingPeer] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, TestResult>>({});
  const [testing, setTesting] = useState<Record<string, boolean>>({});

  const [formData, setFormData] = useState(emptyFormData);

  const admin = isAdmin();

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

  const openCreateForm = () => {
    setFormData(emptyFormData);
    setEditingPeer(null);
    setShowForm(true);
  };

  const openEditForm = (peer: Peer) => {
    setFormData({
      name: peer.name,
      host: peer.host,
      port: peer.port,
      user: peer.user,
      password: '', // Don't show password
      database: peer.database,
      ssl_mode: peer.ssl_mode,
    });
    setEditingPeer(peer.name);
    setShowForm(true);
  };

  const closeForm = () => {
    setShowForm(false);
    setEditingPeer(null);
    setFormData(emptyFormData);
  };

  const savePeer = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const endpoint = editingPeer
        ? `/v1/peers/${editingPeer}`
        : `/v1/peers`;
      const method = editingPeer ? 'PUT' : 'POST';

      const res = await authFetch(endpoint, {
        method,
        body: JSON.stringify(formData),
      });
      if (!res.ok) throw new Error(`Failed to ${editingPeer ? 'update' : 'create'} peer`);
      closeForm();
      fetchPeers();
    } catch (err) {
      console.error(`Failed to ${editingPeer ? 'update' : 'create'} peer:`, err);
    }
  };

  const deletePeer = async (name: string) => {
    if (!confirm(`Are you sure you want to delete peer "${name}"?`)) return;
    try {
      const res = await authFetch(`/v1/peers/${name}`, {
        method: 'DELETE',
      });
      if (!res.ok) throw new Error('Failed to delete peer');
      fetchPeers();
    } catch (err) {
      console.error('Failed to delete peer:', err);
    }
  };

  const testPeer = async (name: string) => {
    setTesting(prev => ({ ...prev, [name]: true }));
    try {
      const res = await authFetch(`/v1/peers/${name}/test`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error('Failed to test peer');
      const result = await res.json();
      setTestResults(prev => ({ ...prev, [name]: result }));
    } catch (err) {
      setTestResults(prev => ({
        ...prev,
        [name]: { success: false, error: 'Failed to test connection' },
      }));
    } finally {
      setTesting(prev => ({ ...prev, [name]: false }));
    }
  };

  useEffect(() => {
    fetchPeers();
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Peers</h1>
        {admin && (
          <button
            id="add-peer-btn"
            onClick={openCreateForm}
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
          >
            <Plus className="w-4 h-4" />
            Add Peer
          </button>
        )}
      </div>

      {/* Add/Edit Peer Form */}
      {showForm && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl dark:shadow-gray-900/40 max-w-md w-full">
            <div className="p-6 border-b dark:border-gray-700">
              <h2 className="text-xl font-bold dark:text-white">
                {editingPeer ? 'Edit Peer Connection' : 'Add Peer Connection'}
              </h2>
            </div>
            <form onSubmit={savePeer} className="p-6 space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Name</label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                  placeholder="my-postgres-source"
                  required
                  disabled={!!editingPeer}
                />
                {editingPeer && (
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">Name cannot be changed</p>
                )}
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Host</label>
                  <input
                    type="text"
                    value={formData.host}
                    onChange={(e) => setFormData({ ...formData, host: e.target.value })}
                    className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                    placeholder="host.docker.internal"
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Port</label>
                  <input
                    type="number"
                    value={formData.port}
                    onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) })}
                    className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                    required
                  />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">User</label>
                  <input
                    type="text"
                    value={formData.user}
                    onChange={(e) => setFormData({ ...formData, user: e.target.value })}
                    className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">
                    Password {editingPeer && <span className="text-gray-400 dark:text-gray-500">(leave blank to keep)</span>}
                  </label>
                  <input
                    type="password"
                    value={formData.password}
                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                    className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                    placeholder={editingPeer ? '••••••••' : ''}
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">Database</label>
                <input
                  type="text"
                  value={formData.database}
                  onChange={(e) => setFormData({ ...formData, database: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                  placeholder="mydb"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200">SSL Mode</label>
                <select
                  value={formData.ssl_mode}
                  onChange={(e) => setFormData({ ...formData, ssl_mode: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 border p-2 dark:bg-gray-800 dark:text-white"
                >
                  <option value="disable">Disable</option>
                  <option value="prefer">Prefer</option>
                  <option value="require">Require</option>
                  <option value="verify-ca">Verify CA</option>
                  <option value="verify-full">Verify Full</option>
                </select>
              </div>
              <div className="flex justify-end gap-2 pt-4">
                <button
                  type="button"
                  onClick={closeForm}
                  className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg dark:text-gray-300 dark:hover:bg-gray-800"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
                >
                  {editingPeer ? 'Save Changes' : 'Create Peer'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Peers List */}
      {peers.length === 0 ? (
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-8 text-center">
          <Database className="w-12 h-12 mx-auto text-gray-400 dark:text-gray-500 mb-4" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">No peers yet</h3>
          <p className="text-gray-500 dark:text-gray-400">Add a peer connection to get started with replication.</p>
        </div>
      ) : (
        <div id="peers-list" className="grid gap-4">
          {peers.map((peer) => (
            <div key={peer.id} className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-6">
              <div className="flex justify-between items-start">
                <div className="flex items-center gap-3">
                  <Database className="w-6 h-6 text-blue-500" />
                  <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{peer.name}</h3>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                      {peer.user}@{peer.host}:{peer.port}/{peer.database}
                    </p>
                  </div>
                </div>
                {admin && (
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => testPeer(peer.name)}
                      disabled={testing[peer.name]}
                      className="flex items-center gap-1 px-3 py-1.5 bg-green-100 text-green-700 rounded hover:bg-green-200 disabled:opacity-50 dark:bg-green-900/30 dark:text-green-400 dark:hover:bg-green-900/50"
                    >
                      {testing[peer.name] ? (
                        <Loader2 className="w-4 h-4 animate-spin" />
                      ) : (
                        <TestTube className="w-4 h-4" />
                      )}
                      Test
                    </button>
                    <button
                      onClick={() => openEditForm(peer)}
                      className="flex items-center gap-1 px-3 py-1.5 bg-blue-100 text-blue-700 rounded hover:bg-blue-200 dark:bg-blue-900/30 dark:text-blue-400 dark:hover:bg-blue-900/50"
                    >
                      <Pencil className="w-4 h-4" />
                      Edit
                    </button>
                    <button
                      onClick={() => deletePeer(peer.name)}
                      className="flex items-center gap-1 px-3 py-1.5 bg-red-100 text-red-700 rounded hover:bg-red-200 dark:bg-red-900/30 dark:text-red-400 dark:hover:bg-red-900/50"
                    >
                      <Trash2 className="w-4 h-4" />
                      Delete
                    </button>
                  </div>
                )}
              </div>

              {testResults[peer.name] && (
                <div className={`mt-4 p-3 rounded ${
                  testResults[peer.name].success
                    ? 'bg-green-50 border border-green-200 dark:bg-green-900/20 dark:border-green-800'
                    : 'bg-red-50 border border-red-200 dark:bg-red-900/20 dark:border-red-800'
                }`}>
                  <div className="flex items-center gap-2">
                    {testResults[peer.name].success ? (
                      <>
                        <CheckCircle className="w-5 h-5 text-green-500" />
                        <span className="text-green-700 dark:text-green-400">Connection successful</span>
                      </>
                    ) : (
                      <>
                        <XCircle className="w-5 h-5 text-red-500" />
                        <span className="text-red-700 dark:text-red-400">Connection failed</span>
                      </>
                    )}
                  </div>
                  {testResults[peer.name].version && (
                    <p className="text-sm text-gray-600 dark:text-gray-300 mt-1 font-mono">
                      {testResults[peer.name].version}
                    </p>
                  )}
                  {testResults[peer.name].error && (
                    <p className="text-sm text-red-600 dark:text-red-400 mt-1">{testResults[peer.name].error}</p>
                  )}
                </div>
              )}

              <div className="mt-4 grid grid-cols-2 gap-4 text-sm">
                <div>
                  <span className="text-gray-500 dark:text-gray-400">SSL Mode:</span>
                  <span className="ml-2 font-medium">{peer.ssl_mode}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
