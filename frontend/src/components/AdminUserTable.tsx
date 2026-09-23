import { Shield, Trash2, Edit2, User as UserIcon } from 'lucide-react';
import type { AdminUser } from '../services/types';

interface AdminUserTableProps {
  users: AdminUser[];
  onEdit: (user: AdminUser) => void;
  onDelete: (id: number) => void;
}

const AdminUserTable = ({ users, onEdit, onDelete }: AdminUserTableProps) => {
  return (
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
                        onClick={() => onEdit(user)}
                        className="p-2 text-zinc-500 hover:text-blue-400 hover:bg-blue-500/10 rounded-lg transition-colors inline-flex"
                        aria-label={`Edit user ${user.email}`}
                        title="Edit User"
                      >
                        <Edit2 className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => onDelete(user.id)}
                        className="p-2 text-zinc-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-colors inline-flex"
                        aria-label={`Delete user ${user.email}`}
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
  );
};

export default AdminUserTable;
