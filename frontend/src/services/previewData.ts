import type { AdminUser, Host, SessionUser } from './types';

export type PreviewScenario = '1' | '2' | '3' | '4';

// Development-only mock scenarios: ?preview=1..4; add &panel=admin to open user administration.
export const previewScenario: PreviewScenario = (() => {
  if (typeof window === 'undefined') return '1';
  const requested = new URLSearchParams(window.location.search).get('preview');
  return requested === '2' || requested === '3' || requested === '4' ? requested : '1';
})();

export const previewUser: SessionUser = {
  id: 1,
  email: 'admin@example.com',
  is_admin: true,
};

const standardPreviewUser: SessionUser = { id: 2, email: 'alex@example.com', is_admin: false };
export const currentPreviewUser = previewScenario === '2' || previewScenario === '4' ? standardPreviewUser : previewUser;

const initialHosts: Host[] = [
  { ID: 'studio-pc', Name: 'Studio PC', IP: '192.0.2.10', MAC: '02:00:00:00:00:10', online: true, last_pinged: '' },
  { ID: 'media-server', Name: 'Media server', IP: '192.0.2.20', MAC: '02:00:00:00:00:20', online: false, last_pinged: '' },
  { ID: 'workstation', Name: 'Workstation', IP: '192.0.2.30', MAC: '02:00:00:00:00:30', online: true, last_pinged: '' },
];

const initialUsers: AdminUser[] = [
  { ...previewUser, devices: [] },
  { id: 2, email: 'alex@example.com', is_admin: false, devices: [{ id: 1, user_id: 2, device_id: 'studio-pc' }, { id: 2, user_id: 2, device_id: 'media-server' }] },
  { id: 3, email: 'sam@example.com', is_admin: false, devices: [{ id: 3, user_id: 3, device_id: 'workstation' }] },
];

let hosts = initialHosts.map(host => ({ ...host }));
let users = initialUsers.map(user => ({ ...user, devices: [...user.devices] }));

export const previewHosts = (): Host[] => {
  const last_pinged = new Date().toLocaleTimeString('en-GB', { hour12: false });
  hosts = hosts.map(host => ({ ...host, last_pinged }));
  const visibleHosts = previewScenario === '4' ? [] : previewScenario === '2'
    ? hosts.filter(host => initialUsers.find(user => user.id === standardPreviewUser.id)?.devices.some(device => device.device_id === host.ID))
    : hosts;
  return visibleHosts.map(host => ({ ...host }));
};
export const previewUsers = (): AdminUser[] => users.map(user => ({ ...user, devices: [...user.devices] }));

export const previewSetHostOnline = (id: string, online: boolean): void => {
  hosts = hosts.map(host => host.ID === id ? { ...host, online } : host);
};

export const previewCreateUser = (email: string, isAdmin: boolean, devices: string[]): void => {
  const id = Math.max(...users.map(user => user.id), 0) + 1;
  users = [...users, {
    id, email, is_admin: isAdmin,
    devices: isAdmin ? [] : devices.map((device_id, index) => ({ id: index + 1, user_id: id, device_id })),
  }];
};

export const previewUpdateUser = (id: number, data: { is_admin?: boolean; devices?: string[] }): void => {
  users = users.map(user => user.id === id ? {
    ...user,
    is_admin: data.is_admin ?? user.is_admin,
    devices: data.is_admin ? [] : (data.devices ?? user.devices.map(device => device.device_id))
      .map((device_id, index) => ({ id: index + 1, user_id: id, device_id })),
  } : user);
};

export const previewDeleteUser = (id: number): void => {
  users = users.filter(user => user.id !== id);
};
