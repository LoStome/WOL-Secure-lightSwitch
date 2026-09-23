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
  devices: { id: number; user_id: number; device_id: string }[];
}

export type ApiErrorKind = 'http' | 'network' | 'timeout' | 'invalid-response';

export class ApiError extends Error {
  readonly kind: ApiErrorKind;
  readonly status?: number;
  readonly requestId?: string;

  constructor(
    message: string,
    kind: ApiErrorKind,
    status?: number,
    requestId?: string,
  ) {
    super(message);
    this.name = 'ApiError';
    this.kind = kind;
    this.status = status;
    this.requestId = requestId;
  }
}

const API_BASE = '/api';
const REQUEST_TIMEOUT_MS = 60_000;

const getHeaders = () => ({
  'Content-Type': 'application/json',
});

interface ApiErrorDetails {
  message?: string;
  requestId?: string;
}

interface RequestOptions {
  parseJson?: boolean;
  allowUnauthorized?: boolean;
  reloadOnUnauthorized?: boolean;
}

const getStatusMessage = (status: number): string => {
  if (status === 401) return 'Authentication failed. Please sign in again.';
  if (status === 403) return 'You are not authorized to perform this action.';
  if (status === 409) return 'The request conflicts with the current server state.';
  if (status >= 500) return 'The server could not complete the request.';
  if (status >= 400) return `The request was rejected (HTTP ${status}).`;
  return `The request failed (HTTP ${status}).`;
};

const readErrorDetails = async (response: Response): Promise<ApiErrorDetails> => {
  try {
    const body: unknown = await response.json();
    if (typeof body !== 'object' || body === null) return {};

    const record = body as Record<string, unknown>;
    const message = typeof record.error === 'string' && record.error.trim()
      ? record.error
      : typeof record.message === 'string' && record.message.trim()
        ? record.message
        : undefined;
    const requestId = typeof record.request_id === 'string' && record.request_id.trim()
      ? record.request_id
      : undefined;

    return { message, requestId };
  } catch {
    return {};
  }
};

const createApiError = async (response: Response): Promise<ApiError> => {
  const details = await readErrorDetails(response);
  const message = details.message ?? getStatusMessage(response.status);
  const messageWithRequestId = details.requestId
    ? `${message} (Request ID: ${details.requestId})`
    : message;

  return new ApiError(messageWithRequestId, 'http', response.status, details.requestId);
};

const request = async <T>(
  path: string,
  init: RequestInit = {},
  options: RequestOptions = {},
): Promise<T> => {
  const timeoutController = new AbortController();
  const callerSignal = init.signal;
  let timedOut = false;
  const abortForTimeout = () => {
    timedOut = true;
    timeoutController.abort();
  };
  const abortForCaller = () => timeoutController.abort();
  const timeout = globalThis.setTimeout(abortForTimeout, REQUEST_TIMEOUT_MS);

  if (callerSignal?.aborted) {
    abortForCaller();
  } else {
    callerSignal?.addEventListener('abort', abortForCaller, { once: true });
  }

  try {
    const response = await fetch(path, { ...init, signal: timeoutController.signal });
    if (!response.ok) {
      if (options.allowUnauthorized && response.status === 401) return undefined as T;

      const error = await createApiError(response);
      if (timedOut) {
        throw new ApiError('The request timed out. Please try again.', 'timeout');
      }
      if (options.reloadOnUnauthorized && response.status === 401) {
        window.location.reload();
      }
      throw error;
    }

    if (options.parseJson === false) return undefined as T;

    try {
      return await response.json() as T;
    } catch {
      if (timedOut) {
        throw new ApiError('The request timed out. Please try again.', 'timeout');
      }
      throw new ApiError('The server returned an invalid response.', 'invalid-response', response.status);
    }
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (callerSignal?.aborted) throw error;
    if (timedOut) {
      throw new ApiError('The request timed out. Please try again.', 'timeout');
    }
    throw new ApiError('Unable to reach the server. Check your connection and try again.', 'network');
  } finally {
    globalThis.clearTimeout(timeout);
    callerSignal?.removeEventListener('abort', abortForCaller);
  }
};

export const login = async (email: string, password: string): Promise<{ user: User }> => {
  return request(`${API_BASE}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
};

export const getCurrentUser = async (): Promise<User> => {
  return request(`${API_BASE}/session`);
};

export const logout = async (): Promise<void> => {
  return request(`${API_BASE}/logout`, {
    method: 'POST',
    headers: getHeaders(),
  }, { parseJson: false, allowUnauthorized: true });
};

export const checkSetup = async (): Promise<{ needs_setup: boolean }> => {
  return request(`${API_BASE}/setup`);
};

export const fetchHosts = async (signal?: AbortSignal): Promise<Host[]> => {
  return request(`${API_BASE}/hosts`, { headers: getHeaders(), signal }, { reloadOnUnauthorized: true });
};

export const wakeHost = async (id: string): Promise<void> => {
  return request(`${API_BASE}/wol/${id}`, {
    method: 'POST',
    headers: getHeaders(),
  }, { parseJson: false, reloadOnUnauthorized: true });
};

export const shutdownHost = async (id: string): Promise<void> => {
  return request(`${API_BASE}/shutdown/${id}`, {
    method: 'POST',
    headers: getHeaders(),
  }, { parseJson: false, reloadOnUnauthorized: true });
};

export const fetchUsers = async (): Promise<User[]> => {
  return request(`${API_BASE}/users`, { headers: getHeaders() }, { reloadOnUnauthorized: true });
};

export const createUser = async (
  email: string,
  password: string,
  isAdmin: boolean,
  devices: string[],
): Promise<void> => {
  return request(`${API_BASE}/users`, {
    method: 'POST',
    headers: getHeaders(),
    body: JSON.stringify({ email, password, is_admin: isAdmin, devices }),
  }, { parseJson: false, reloadOnUnauthorized: true });
};

export const deleteUser = async (id: number): Promise<void> => {
  return request(`${API_BASE}/users/${id}`, {
    method: 'DELETE',
    headers: getHeaders(),
  }, { parseJson: false, reloadOnUnauthorized: true });
};

export const updateUser = async (
  id: number,
  data: { password?: string; is_admin?: boolean; devices?: string[] },
): Promise<void> => {
  return request(`${API_BASE}/users/${id}`, {
    method: 'PUT',
    headers: getHeaders(),
    body: JSON.stringify(data),
  }, { parseJson: false, reloadOnUnauthorized: true });
};
