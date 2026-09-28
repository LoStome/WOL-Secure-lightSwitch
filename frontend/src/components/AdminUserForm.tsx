import type { FormEventHandler } from 'react';
import { Plus, Save, X } from 'lucide-react';
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

const AdminUserForm = ({ editingUserId, email, password, isAdmin, selectedDevices, hosts, submitLoading, resetForm, onSubmit, onEmailChange, onPasswordChange, onAdminChange, onDeviceToggle }: AdminUserFormProps) => <section className="panel form-panel" aria-labelledby="user-form-title">
  <div className="panel-heading"><div><h2 id="user-form-title">{editingUserId ? 'Edit user' : 'Add a person'}</h2><p>{editingUserId ? 'Update role, password, or device access.' : 'Give someone access to selected devices.'}</p></div>
    {editingUserId && <button className="icon-button" onClick={resetForm} aria-label="Cancel editing" title="Cancel editing"><X size={18} aria-hidden="true" /></button>}</div>
  <form onSubmit={onSubmit} className="user-form">
    <div className="field"><label htmlFor="admin-email">Email</label><input id="admin-email" type="email" value={email} onChange={e => onEmailChange(e.target.value)} required disabled={!!editingUserId} maxLength={254} placeholder="name@example.com" /></div>
    <div className="field"><label htmlFor="admin-password">Password</label><input id="admin-password" type="password" value={password} onChange={e => onPasswordChange(e.target.value)} required={!editingUserId} minLength={12} maxLength={72} placeholder={editingUserId ? 'Leave blank to keep current' : 'Choose a password'} />
      <p className="field-help">{editingUserId ? 'Leave blank to keep the current password.' : 'Use at least 12 characters.'}</p></div>
    <label className="admin-check"><input type="checkbox" checked={isAdmin} onChange={e => onAdminChange(e.target.checked)} /><span><strong>Administrator</strong><small>Can manage people and access all devices.</small></span></label>
    {!isAdmin && <fieldset className="device-fieldset"><legend>Allowed devices</legend><p>Select the devices this person can see and control.</p>
      {hosts.length === 0 ? <p>No devices configured.</p> : <div className="device-options">{hosts.map(host => <label key={host.ID} className="device-option"><input type="checkbox" checked={selectedDevices.includes(host.ID)} onChange={() => onDeviceToggle(host.ID)} /><span>{host.Name}</span></label>)}</div>}
    </fieldset>}
    <button type="submit" className="primary-button form-submit" disabled={submitLoading || (!email || (!password && !editingUserId))}>{editingUserId ? <Save size={17} aria-hidden="true" /> : <Plus size={17} aria-hidden="true" />}{submitLoading ? 'Saving…' : editingUserId ? 'Save changes' : 'Create user'}</button>
  </form>
</section>;

export default AdminUserForm;
