export interface Host {
  ID: string;
  Name: string;
  MAC: string;
  IP: string;
  online: boolean;
  last_pinged: string;
}

export interface User {
  id: number;
  email: string;
  is_admin: boolean;
  devices: { id: number, user_id: number, device_id: string }[];
}

const API_BASE = "/api";

const getHeaders = () => ({
  "Content-Type": "application/json"
});

const handleAuthError = (response: Response) => {
  if (response.status === 401) {
    window.location.reload();
  }
};

export const login = async (email: string, password: string): Promise<{user: User}> => {
  const response = await fetch(`${API_BASE}/login`, {
    method: 'POST',
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password })
  });
  if (!response.ok) {
    throw new Error('Invalid credentials');
  }
  return response.json();
}

export const getCurrentUser = async (): Promise<User> => {
  const response = await fetch(`${API_BASE}/session`);
  if (!response.ok) {
    throw new Error('Not authenticated');
  }
  return response.json();
};

export const logout = async (): Promise<void> => {
  const response = await fetch(`${API_BASE}/logout`, {
    method: 'POST',
    headers: getHeaders()
  });
  if (!response.ok && response.status !== 401) {
    throw new Error('Failed to revoke session');
  }
}

export const checkSetup = async (): Promise<{needs_setup: boolean}> => {
  const response = await fetch(`${API_BASE}/setup`);
  if (!response.ok) {
    throw new Error('Failed to check setup status');
  }
  return response.json();
}

export const fetchHosts = async (): Promise<Host[]> => {
  const response = await fetch(`${API_BASE}/hosts`, { headers: getHeaders() });
  if (!response.ok) {
    handleAuthError(response);
    throw new Error('Failed to fetch hosts');
  }
  return response.json();
};

export const wakeHost = async (id: string): Promise<void> => {
  const response = await fetch(`${API_BASE}/wol/${id}`, { method: 'POST', headers: getHeaders() });
  if (!response.ok) {
    handleAuthError(response);
    throw new Error('Failed to wake host');
  }
};

export const shutdownHost = async (id: string): Promise<void> => {
  const response = await fetch(`${API_BASE}/shutdown/${id}`, { method: 'POST', headers: getHeaders() });
  if (!response.ok) {
    handleAuthError(response);
    throw new Error('Failed to shutdown host');
  }
};

export const fetchUsers = async (): Promise<User[]> => {
  const response = await fetch(`${API_BASE}/users`, { headers: getHeaders() });
  if (!response.ok) {
    handleAuthError(response);
    throw new Error('Failed to fetch users');
  }
  return response.json();
}

export const createUser = async (email: string, password: string, isAdmin: boolean, devices: string[]): Promise<void> => {
  const response = await fetch(`${API_BASE}/users`, {
    method: 'POST',
    headers: getHeaders(),
    body: JSON.stringify({ email, password, is_admin: isAdmin, devices })
  });
  if (!response.ok) {
    handleAuthError(response);
    throw new Error('Failed to create user');
  }
}

export const deleteUser = async (id: number): Promise<void> => {
  const response = await fetch(`${API_BASE}/users/${id}`, {
    method: 'DELETE',
    headers: getHeaders()
  });
  if (!response.ok) {
    handleAuthError(response);
    throw new Error('Failed to delete user');
  }
}

export const updateUser = async (id: number, data: { password?: string, is_admin?: boolean, devices?: string[] }): Promise<void> => {
  const response = await fetch(`${API_BASE}/users/${id}`, {
    method: 'PUT',
    headers: getHeaders(),
    body: JSON.stringify(data)
  });
  if (!response.ok) {
    handleAuthError(response);
    throw new Error('Failed to update user');
  }
}
