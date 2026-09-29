import { Pencil, Trash2, Shield } from 'lucide-react';
import type { AdminUser } from '../services/types';

interface AdminUserTableProps {
  users: AdminUser[];
  onEdit: (user: AdminUser) => void;
  onDelete: (id: number) => void;
}

const AdminUserTable = ({ users, onEdit, onDelete }: AdminUserTableProps) => <section className="panel user-panel" aria-labelledby="users-title">
  <div className="panel-heading"><div><h2 id="users-title">People</h2><p>Accounts with access to this installation</p></div><span className="count-label">{users.length}</span></div>
  {users.length === 0 ? <p className="list-empty">No users found.</p> : <div className="user-list">
    {users.map(user => <div className="user-row" key={user.id}>
      <div className="user-identity"><span className="user-avatar" aria-hidden="true">{user.email.charAt(0).toUpperCase()}</span><div><strong>{user.email}</strong><span>Account #{user.id}</span></div></div>
      <div className="user-access">{user.is_admin ? <span className="access-badge"><Shield size={13} aria-hidden="true" /> Administrator</span>
        : user.devices?.length ? <span>{user.devices.length} {user.devices.length === 1 ? 'device' : 'devices'} assigned</span> : <span>No devices assigned</span>}</div>
      <div className="row-actions"><button className="icon-button" onClick={() => onEdit(user)} aria-label={`Edit user ${user.email}`} title="Edit user"><Pencil size={17} aria-hidden="true" /></button>
        <button className="icon-button danger-icon" onClick={() => onDelete(user.id)} aria-label={`Delete user ${user.email}`} title="Delete user"><Trash2 size={17} aria-hidden="true" /></button></div>
    </div>)}
  </div>}
</section>;

export default AdminUserTable;
