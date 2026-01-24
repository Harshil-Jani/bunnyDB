const API_URL = process.env.NEXT_PUBLIC_API_URL || process.env.BUNNY_API_URL || 'http://localhost:8112';

export interface AuthUser {
  username: string;
  role: 'admin' | 'readonly';
}

export function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('bunny_token');
}

export function setToken(token: string) {
  localStorage.setItem('bunny_token', token);
}

export function clearToken() {
  localStorage.removeItem('bunny_token');
}

export function getUser(): AuthUser | null {
  const token = getToken();
  if (!token) return null;
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    return { username: payload.username, role: payload.role };
  } catch {
    return null;
  }
}

export function isAdmin(): boolean {
  const user = getUser();
  return user?.role === 'admin';
}

export async function authFetch(
  endpoint: string,
  options?: RequestInit & { skipAuthRedirect?: boolean }
): Promise<Response> {
  const token = getToken();
  const { skipAuthRedirect, ...fetchOptions } = options || {};
  const res = await fetch(`${API_URL}${endpoint}`, {
    ...fetchOptions,
    headers: {
      'Content-Type': 'application/json',
      ...(token && { 'Authorization': `Bearer ${token}` }),
      ...fetchOptions?.headers,
    },
  });
  if (res.status === 401 && !skipAuthRedirect) {
    clearToken();
    window.location.href = '/login';
  }
  return res;
}

export async function login(username: string, password: string): Promise<AuthUser> {
  const res = await fetch(`${API_URL}/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || 'Invalid credentials');
  }
  const data = await res.json();
  setToken(data.token);
  return { username: data.username, role: data.role };
}

export function logout() {
  clearToken();
  window.location.href = '/login';
}
