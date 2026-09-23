import React, { useState, useEffect } from 'react';
import { fetchUsers, createUser, updateUser, deleteUser, fetchHosts } from '../services/api';
import type { AdminUser, Host } from '../services/types';
import AdminUserForm from './AdminUserForm';
import AdminUserTable from './AdminUserTable';
import { getErrorMessage } from '../utils/errorMessage';
import { Loader2 } from 'lucide-react';

const AdminPanel = () => {
  const [users, setUsers] = useState<AdminUser[]>([]);
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
    } catch (err: unknown) {
      setError('Failed to load admin data: ' + getErrorMessage(err));
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

  const handleEditClick = (user: AdminUser) => {
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
        const updateData: { is_admin: boolean; devices: string[]; password?: string } = {
          is_admin: isAdmin,
          devices: selectedDevices
        };
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
    } catch (err: unknown) {
      setError(`Failed to ${editingUserId ? 'update' : 'create'} user: ` + getErrorMessage(err));
    } finally {
      setSubmitLoading(false);
    }
  };

  const handleDeleteUser = async (id: number) => {
    if (!window.confirm('Are you sure you want to delete this user?')) return;
    try {
      await deleteUser(id);
      await loadData();
    } catch (err: unknown) {
      setError('Failed to delete user: ' + getErrorMessage(err));
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
        <div role="alert" className="p-4 bg-red-500/10 border border-red-500/20 text-red-400 rounded-xl">
          {error}
        </div>
      )}

      <AdminUserForm
        editingUserId={editingUserId}
        email={email}
        password={password}
        isAdmin={isAdmin}
        selectedDevices={selectedDevices}
        hosts={hosts}
        submitLoading={submitLoading}
        resetForm={resetForm}
        onSubmit={handleSubmitUser}
        onEmailChange={setEmail}
        onPasswordChange={setPassword}
        onAdminChange={(checked) => {
          setIsAdmin(checked);
          if (checked) setSelectedDevices([]); // Admins get all automatically
        }}
        onDeviceToggle={handleDeviceToggle}
      />

      <AdminUserTable users={users} onEdit={handleEditClick} onDelete={handleDeleteUser} />

    </div>
  );
};

export default AdminPanel;
