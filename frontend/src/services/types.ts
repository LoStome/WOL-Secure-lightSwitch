export interface Host {
  ID: string;
  Name: string;
  MAC: string;
  IP: string;
  online: boolean;
  last_pinged: string;
}

export interface SessionUser {
  id: number;
  email: string;
  is_admin: boolean;
}

export interface AdminUser extends SessionUser {
  devices: { id: number; user_id: number; device_id: string }[];
}
