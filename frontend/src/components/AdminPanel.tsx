import React, { useState, useEffect } from 'react';
import { fetchUsers, createUser, updateUser, deleteUser, fetchHosts } from '../services/api';
import type { User, Host } from '../services/api';
import { Shield, Trash2, Edit2, Plus, Loader2, User as UserIcon, X } from 'lucide-react';

const AdminPanel = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Form state
  const [editingUserId, setEditingUserId] = useState<number | null>(null);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isAdmin, setIsAdmin] = useState(false);
  const [selectedDevices, setSelectedDevices] = useState<string[]>([]);
  const [submitLoading, setSubmitLoading] = useState(false);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      const [usersData, hostsData] = await Promise.all([
        fetchUsers(),
        fetchHosts()
      ]);
      setUsers(usersData);
      setHosts(hostsData);
    } catch (err: any) {
      setError('Failed to load admin data: ' + err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleDeviceToggle = (deviceId: string) => {
    setSelectedDevices(prev => 
      prev.includes(deviceId) 
        ? prev.filter(id => id !== deviceId)
        : [...prev, deviceId]
    );
  };

  const resetForm = () => {
    setEditingUserId(null);
    setEmail('');
    setPassword('');
    setIsAdmin(false);
    setSelectedDevices([]);
    setError('');
  };

  const handleEditClick = (user: User) => {
    setEditingUserId(user.id);
    setEmail(user.email);
    setPassword(''); // Leave blank intentionally
    setIsAdmin(user.is_admin);
    setSelectedDevices(user.devices ? user.devices.map(d => d.device_id) : []);
    
    // Scroll to top
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const handleSubmitUser = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitLoading(true);
    setError('');
    try {
      if (editingUserId) {
        // Edit mode
        const updateData: any = { is_admin: isAdmin, devices: selectedDevices };
        if (password) {
          updateData.password = password;
        }
        await updateUser(editingUserId, updateData);
      } else {
        // Create mode
        await createUser(email, password, isAdmin, selectedDevices);
      }
      resetForm();
      await loadData();
    } catch (err: any) {
      setError(`Failed to ${editingUserId ? 'update' : 'create'} user: ` + err.message);
    } finally {
      setSubmitLoading(false);
    }
  };

  const handleDeleteUser = async (id: number) => {
    if (!window.confirm('Are you sure you want to delete this user?')) return;
    try {
      await deleteUser(id);
      await loadData();
    } catch (err: any) {
      setError('Failed to delete user: ' + err.message);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[50vh]">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500 pb-10">
      
      {error && (
        <div className="p-4 bg-red-500/10 border border-red-500/20 text-red-400 rounded-xl">
          {error}
        </div>
      )}

      {/* User Form */}
      <div className={`bg-zinc-900/50 backdrop-blur-xl border ${editingUserId ? 'border-amber-500/50 shadow-[0_0_15px_rgba(245,158,11,0.1)]' : 'border-zinc-800'} rounded-3xl p-6 sm:p-8 transition-colors duration-300`}>
        <div className="flex justify-between items-center mb-6">
          <h2 className={`text-xl font-semibold flex items-center gap-2 ${editingUserId ? 'text-amber-400' : 'text-zinc-100'}`}>
            {editingUserId ? (
              <><Edit2 className="w-5 h-5" /> Edit User: {email}</>
            ) : (
              <><Plus className="w-5 h-5 text-blue-400" /> Add New User</>
            )}
          </h2>
          {editingUserId && (
            <button 
              onClick={resetForm} 
              className="text-zinc-400 hover:text-white flex items-center gap-1 text-sm bg-zinc-800/50 hover:bg-zinc-800 px-3 py-1.5 rounded-lg transition-colors border border-zinc-700/50"
            >
              <X className="w-4 h-4" /> Cancel Edit
            </button>
          )}
        </div>

        <form onSubmit={handleSubmitUser} className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-2">
              <label className="text-xs font-semibold text-zinc-400 uppercase tracking-wider ml-1">Email</label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={!!editingUserId} // Don't allow email change for now
                className={`w-full px-4 py-2.5 bg-zinc-950 border border-zinc-800 rounded-xl text-zinc-200 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500 transition-all ${editingUserId ? 'opacity-50 cursor-not-allowed' : ''}`}
                placeholder="user@example.com"
              />
            </div>
            
            <div className="space-y-2">
              <label className="text-xs font-semibold text-zinc-400 uppercase tracking-wider ml-1">
                Password {editingUserId && <span className="normal-case text-amber-500/70 ml-1">(Leave blank to keep current)</span>}
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required={!editingUserId}
                className="w-full px-4 py-2.5 bg-zinc-950 border border-zinc-800 rounded-xl text-zinc-200 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500 transition-all"
                placeholder={editingUserId ? "•••••••• (unchanged)" : "••••••••"}
              />
            </div>
          </div>

          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="isAdmin"
              checked={isAdmin}
              onChange={(e) => {
                setIsAdmin(e.target.checked);
                if (e.target.checked) setSelectedDevices([]); // Admins get all automatically
              }}
              className="w-5 h-5 rounded border-zinc-700 text-blue-500 focus:ring-blue-500 focus:ring-offset-zinc-900 bg-zinc-950"
            />
            <label htmlFor="isAdmin" className="text-sm font-medium text-zinc-300 cursor-pointer select-none flex items-center gap-2">
              <Shield className="w-4 h-4 text-amber-400" />
              Make this user an Administrator (has access to all devices by default)
            </label>
          </div>

          {!isAdmin && hosts.length > 0 && (
            <div className="space-y-3 pt-2">
              <label className="text-xs font-semibold text-zinc-400 uppercase tracking-wider ml-1 flex items-center justify-between">
                <span>Allowed Devices</span>
                <span className="text-zinc-500 font-normal normal-case">Select which hosts this user can see and control</span>
              </label>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                {hosts.map(host => (
                  <label 
                    key={host.ID}
                    className={`flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-colors ${
                      selectedDevices.includes(host.ID) 
                        ? 'bg-blue-500/10 border-blue-500/50 text-blue-300' 
                        : 'bg-zinc-950/50 border-zinc-800 text-zinc-400 hover:border-zinc-700'
                    }`}
                  >
                    <input
                      type="checkbox"
                      className="mt-1"
                      checked={selectedDevices.includes(host.ID)}
                      onChange={() => handleDeviceToggle(host.ID)}
                    />
                    <div>
                      <div className="font-medium text-zinc-200">{host.Name}</div>
                      <div className="text-xs opacity-70 font-mono mt-0.5">{host.ID}</div>
                    </div>
                  </label>
                ))}
              </div>
            </div>
          )}

          <div className="pt-4 flex justify-end">
            <button
              type="submit"
              disabled={submitLoading || (!email || (!password && !editingUserId))}
              className={`flex items-center gap-2 py-2 px-6 ${editingUserId ? 'bg-amber-600 hover:bg-amber-500' : 'bg-blue-600 hover:bg-blue-500'} text-white font-medium rounded-xl transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-zinc-900 disabled:opacity-50 disabled:cursor-not-allowed`}
            >
              {submitLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : (editingUserId ? <Edit2 className="w-4 h-4" /> : <Plus className="w-4 h-4" />)}
              {editingUserId ? 'Save Changes' : 'Create User'}
            </button>
          </div>
        </form>
      </div>

      {/* Users List */}
      <div className="bg-zinc-900/50 backdrop-blur-xl border border-zinc-800 rounded-3xl p-6 sm:p-8 overflow-hidden">
        <h2 className="text-xl font-semibold text-zinc-100 flex items-center gap-2 mb-6">
          <UserIcon className="w-5 h-5 text-zinc-400" />
          Manage Users
        </h2>

        <div className="overflow-x-auto -mx-6 sm:mx-0">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-zinc-800 text-xs font-semibold text-zinc-500 uppercase tracking-wider">
                <th className="px-6 sm:px-4 py-3">ID</th>
                <th className="px-6 sm:px-4 py-3">Email</th>
                <th className="px-6 sm:px-4 py-3">Role & Access</th>
                <th className="px-6 sm:px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-800/50">
              {users.map(user => (
                <tr key={user.id} className="hover:bg-zinc-800/20 transition-colors">
                  <td className="px-6 sm:px-4 py-4 text-sm font-mono text-zinc-500">#{user.id}</td>
                  <td className="px-6 sm:px-4 py-4 whitespace-nowrap">
                    <div className="font-medium text-zinc-200">{user.email}</div>
                  </td>
                  <td className="px-6 sm:px-4 py-4">
                    {user.is_admin ? (
                      <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">
                        <Shield className="w-3 h-3" />
                        Admin
                      </span>
                    ) : (
                      <div className="flex flex-wrap gap-1.5">
                        {user.devices?.length > 0 ? (
                          user.devices.map(d => (
                            <span key={d.device_id} className="px-2 py-0.5 rounded text-xs font-mono bg-zinc-800 text-zinc-400 border border-zinc-700">
                              {d.device_id}
                            </span>
                          ))
                        ) : (
                          <span className="text-xs text-zinc-600 italic">No devices assigned</span>
                        )}
                      </div>
                    )}
                  </td>
                  <td className="px-6 sm:px-4 py-4 text-right">
                    <div className="flex justify-end gap-2">
                      <button
                        onClick={() => handleEditClick(user)}
                        className="p-2 text-zinc-500 hover:text-blue-400 hover:bg-blue-500/10 rounded-lg transition-colors inline-flex"
                        title="Edit User"
                      >
                        <Edit2 className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDeleteUser(user.id)}
                        className="p-2 text-zinc-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-colors inline-flex"
                        title="Delete User"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {users.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-6 py-8 text-center text-zinc-500">
                    No users found.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

    </div>
  );
};

export default AdminPanel;
