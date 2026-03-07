export interface Host {
  ID: string;
  Name: string;
  MAC: string;
  IP: string;
}

const API_BASE = "http://localhost:8080/api";

export const fetchHosts = async (): Promise<Host[]> => {
  const response = await fetch(`${API_BASE}/hosts`);
  if (!response.ok) {
    throw new Error('Failed to fetch hosts');
  }
  return response.json();
};

export const wakeHost = async (id: string): Promise<void> => {
  const response = await fetch(`${API_BASE}/wol/${id}`, { method: 'POST' });
  if (!response.ok) {
    throw new Error('Failed to wake host');
  }
};

export const shutdownHost = async (id: string): Promise<void> => {
  const response = await fetch(`${API_BASE}/shutdown/${id}`, { method: 'POST' });
  if (!response.ok) {
    throw new Error('Failed to shutdown host');
  }
};
