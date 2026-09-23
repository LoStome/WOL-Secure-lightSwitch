import type { FormEventHandler } from 'react';
import { Shield, Edit2, Plus, Loader2, X } from 'lucide-react';
import type { Host } from '../services/types';

interface AdminUserFormProps {
  editingUserId: number | null;
  email: string;
  password: string;
  isAdmin: boolean;
  selectedDevices: string[];
  hosts: Host[];
  submitLoading: boolean;
  resetForm: () => void;
  onSubmit: FormEventHandler<HTMLFormElement>;
  onEmailChange: (email: string) => void;
  onPasswordChange: (password: string) => void;
  onAdminChange: (isAdmin: boolean) => void;
  onDeviceToggle: (deviceId: string) => void;
}

const AdminUserForm = ({ editingUserId, email, password, isAdmin, selectedDevices, hosts, submitLoading, resetForm, onSubmit, onEmailChange, onPasswordChange, onAdminChange, onDeviceToggle }: AdminUserFormProps) => {
  return (
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

        <form onSubmit={onSubmit} className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-2">
              <label htmlFor="admin-email" className="text-xs font-semibold text-zinc-400 uppercase tracking-wider ml-1">Email</label>
              <input
                id="admin-email"
                type="email"
                value={email}
                onChange={(e) => onEmailChange(e.target.value)}
                required
                disabled={!!editingUserId} // Don't allow email change for now
                maxLength={254}
                className={`w-full px-4 py-2.5 bg-zinc-950 border border-zinc-800 rounded-xl text-zinc-200 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500 transition-all ${editingUserId ? 'opacity-50 cursor-not-allowed' : ''}`}
                placeholder="user@example.com"
              />
              <p className="text-xs text-zinc-500 ml-1">Use a valid email address, up to 254 bytes.</p>
            </div>

            <div className="space-y-2">
              <label htmlFor="admin-password" className="text-xs font-semibold text-zinc-400 uppercase tracking-wider ml-1">
                Password {editingUserId && <span className="normal-case text-amber-500/70 ml-1">(Leave blank to keep current)</span>}
              </label>
              <input
                id="admin-password"
                type="password"
                value={password}
                onChange={(e) => onPasswordChange(e.target.value)}
                required={!editingUserId}
                minLength={12}
                maxLength={72}
                className="w-full px-4 py-2.5 bg-zinc-950 border border-zinc-800 rounded-xl text-zinc-200 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500 transition-all"
                placeholder={editingUserId ? "•••••••• (unchanged)" : "••••••••"}
              />
              <p className="text-xs text-zinc-500 ml-1">At least 12 characters and at most 72 bytes. Leave blank while editing to keep the current password.</p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="isAdmin"
              checked={isAdmin}
              onChange={(e) => onAdminChange(e.target.checked)}
              className="w-5 h-5 rounded border-zinc-700 text-blue-500 focus:ring-blue-500 focus:ring-offset-zinc-900 bg-zinc-950"
            />
            <label htmlFor="isAdmin" className="text-sm font-medium text-zinc-300 cursor-pointer select-none flex items-center gap-2">
              <Shield className="w-4 h-4 text-amber-400" />
              Make this user an Administrator (has access to all devices by default)
            </label>
          </div>

          {!isAdmin && hosts.length > 0 && (
            <fieldset className="space-y-3 pt-2">
              <legend className="text-xs font-semibold text-zinc-400 uppercase tracking-wider ml-1 flex items-center justify-between">
                <span>Allowed Devices</span>
                <span className="text-zinc-500 font-normal normal-case">Select which hosts this user can see and control</span>
              </legend>
              <p className="text-xs text-zinc-500 ml-1">Device IDs come from hosts.yaml and use 1-64 ASCII characters: letters, numbers, '.', '_' or '-'; the first character must be a letter or number.</p>
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
                      onChange={() => onDeviceToggle(host.ID)}
                    />
                    <div>
                      <div className="font-medium text-zinc-200">{host.Name}</div>
                      <div className="text-xs opacity-70 font-mono mt-0.5">{host.ID}</div>
                    </div>
                  </label>
                ))}
              </div>
            </fieldset>
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
  );
};

export default AdminUserForm;
