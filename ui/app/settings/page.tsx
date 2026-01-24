'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Plus, Trash2, Users, Shield, Eye, KeyRound } from 'lucide-react';
import { authFetch, isAdmin, getUser } from '../../lib/auth';

interface User {
  id: number;
  username: string;
  role: string;
  created_at: string;
}

export default function SettingsPage() {
  const router = useRouter();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState({ username: '', password: '', role: 'readonly' });
  const [formError, setFormError] = useState('');
  const [creating, setCreating] = useState(false);

  // Password change
  const [showPasswordForm, setShowPasswordForm] = useState(false);
  const [passwordData, setPasswordData] = useState({ current_password: '', new_password: '' });
  const [passwordError, setPasswordError] = useState('');
  const [passwordSuccess, setPasswordSuccess] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);

  const admin = isAdmin();
  const currentUser = getUser();

  useEffect(() => {
    if (!admin) {
      // Non-admins can still change their password but can't manage users
      setLoading(false);
      return;
    }
    fetchUsers();
  }, [admin]);

  const fetchUsers = async () => {
    try {
      const res = await authFetch('/v1/users');
      if (res.ok) {
        const data = await res.json();
        setUsers(data || []);
      }
    } catch (err) {
      console.error('Failed to fetch users:', err);
    } finally {
      setLoading(false);
    }
  };

  const createUser = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError('');
    if (!formData.username || !formData.password) {
      setFormError('Username and password are required');
      return;
    }
    setCreating(true);
    try {
      const res = await authFetch('/v1/users', {
        method: 'POST',
        body: JSON.stringify(formData),
      });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to create user');
      }
      setShowForm(false);
      setFormData({ username: '', password: '', role: 'readonly' });
      fetchUsers();
    } catch (err: any) {
      setFormError(err.message || 'Failed to create user');
    } finally {
      setCreating(false);
    }
  };

  const deleteUser = async (username: string) => {
    if (!confirm(`Delete user "${username}"? This cannot be undone.`)) return;
    try {
      const res = await authFetch(`/v1/users/${username}`, { method: 'DELETE' });
      if (!res.ok) {
        const data = await res.json();
        alert(data.error || 'Failed to delete user');
        return;
      }
      fetchUsers();
    } catch (err) {
      console.error('Failed to delete user:', err);
    }
  };

  const changePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError('');
    setPasswordSuccess('');
    if (!passwordData.current_password || !passwordData.new_password) {
      setPasswordError('Both fields are required');
      return;
    }
    setChangingPassword(true);
    try {
      const res = await authFetch('/v1/auth/change-password', {
        method: 'POST',
        body: JSON.stringify(passwordData),
      });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to change password');
      }
      setPasswordSuccess('Password updated successfully');
      setPasswordData({ current_password: '', new_password: '' });
      setTimeout(() => setPasswordSuccess(''), 3000);
    } catch (err: any) {
      setPasswordError(err.message || 'Failed to change password');
    } finally {
      setChangingPassword(false);
    }
  };

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Settings</h1>

      {/* Change Password — available to all users */}
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-6">
        <div className="flex items-center gap-2 mb-4">
          <KeyRound className="w-5 h-5 text-gray-600 dark:text-gray-400" />
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Change Password</h2>
        </div>
        {!showPasswordForm ? (
          <button
            onClick={() => setShowPasswordForm(true)}
            className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
          >
            Change your password
          </button>
        ) : (
          <form onSubmit={changePassword} className="max-w-sm space-y-3">
            {passwordError && (
              <p className="text-sm text-red-600 dark:text-red-400">{passwordError}</p>
            )}
            {passwordSuccess && (
              <p className="text-sm text-green-600 dark:text-green-400">{passwordSuccess}</p>
            )}
            <input
              type="password"
              placeholder="Current password"
              value={passwordData.current_password}
              onChange={(e) => setPasswordData(d => ({ ...d, current_password: e.target.value }))}
              className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
            />
            <input
              type="password"
              placeholder="New password"
              value={passwordData.new_password}
              onChange={(e) => setPasswordData(d => ({ ...d, new_password: e.target.value }))}
              className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
            />
            <div className="flex gap-2">
              <button
                type="submit"
                disabled={changingPassword}
                className="px-4 py-2 bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-sm font-medium rounded-lg hover:bg-gray-800 dark:hover:bg-gray-100 disabled:opacity-50"
              >
                {changingPassword ? 'Updating...' : 'Update Password'}
              </button>
              <button
                type="button"
                onClick={() => { setShowPasswordForm(false); setPasswordError(''); }}
                className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white"
              >
                Cancel
              </button>
            </div>
          </form>
        )}
      </div>

      {/* User Management — admin only */}
      {admin && (
        <div className="bg-white dark:bg-gray-900 rounded-lg shadow dark:shadow-gray-900/20 p-6">
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-2">
              <Users className="w-5 h-5 text-gray-600 dark:text-gray-400" />
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">User Management</h2>
            </div>
            <button
              onClick={() => setShowForm(true)}
              className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600"
            >
              <Plus className="w-4 h-4" />
              Add User
            </button>
          </div>

          {/* Create User Form */}
          {showForm && (
            <div className="mb-6 p-4 border border-gray-200 dark:border-gray-700 rounded-lg">
              <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-3">New User</h3>
              <form onSubmit={createUser} className="space-y-3">
                {formError && (
                  <p className="text-sm text-red-600 dark:text-red-400">{formError}</p>
                )}
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <input
                    type="text"
                    placeholder="Username"
                    value={formData.username}
                    onChange={(e) => setFormData(d => ({ ...d, username: e.target.value }))}
                    className="px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                  />
                  <input
                    type="password"
                    placeholder="Password"
                    value={formData.password}
                    onChange={(e) => setFormData(d => ({ ...d, password: e.target.value }))}
                    className="px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                  />
                  <select
                    value={formData.role}
                    onChange={(e) => setFormData(d => ({ ...d, role: e.target.value }))}
                    className="px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                  >
                    <option value="readonly">Readonly</option>
                    <option value="admin">Admin</option>
                  </select>
                </div>
                <div className="flex gap-2">
                  <button
                    type="submit"
                    disabled={creating}
                    className="px-4 py-2 bg-blue-500 text-white text-sm rounded-lg hover:bg-blue-600 disabled:opacity-50"
                  >
                    {creating ? 'Creating...' : 'Create User'}
                  </button>
                  <button
                    type="button"
                    onClick={() => { setShowForm(false); setFormError(''); }}
                    className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white"
                  >
                    Cancel
                  </button>
                </div>
              </form>
            </div>
          )}

          {/* Users List */}
          {loading ? (
            <p className="text-sm text-gray-500 dark:text-gray-400">Loading...</p>
          ) : (
            <div className="divide-y divide-gray-100 dark:divide-gray-800">
              {users.map((user) => (
                <div key={user.id} className="flex items-center justify-between py-3">
                  <div className="flex items-center gap-3">
                    <div className={`p-1.5 rounded-lg ${user.role === 'admin' ? 'bg-orange-100 dark:bg-orange-900/30' : 'bg-gray-100 dark:bg-gray-800'}`}>
                      {user.role === 'admin' ? (
                        <Shield className="w-4 h-4 text-orange-600 dark:text-orange-400" />
                      ) : (
                        <Eye className="w-4 h-4 text-gray-500 dark:text-gray-400" />
                      )}
                    </div>
                    <div>
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        {user.username}
                        {user.username === currentUser?.username && (
                          <span className="ml-2 text-[10px] uppercase tracking-wider text-gray-400">(you)</span>
                        )}
                      </p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        {user.role} &middot; created {new Date(user.created_at).toLocaleDateString()}
                      </p>
                    </div>
                  </div>
                  {user.username !== currentUser?.username && (
                    <button
                      onClick={() => deleteUser(user.username)}
                      className="p-2 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                      title="Delete user"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
