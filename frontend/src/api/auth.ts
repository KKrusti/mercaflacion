import type { User } from '../types';

const API_BASE = '/api';
const TIMEOUT_MS = 10_000;

function withTimeout(ms: number): { signal: AbortSignal; clear: () => void } {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), ms);
  return { signal: controller.signal, clear: () => clearTimeout(timer) };
}

interface AuthResponse {
  token: string;
  userId: number;
  username: string;
  email?: string;
  isAdmin: boolean;
}

export async function register(username: string, password: string, email?: string): Promise<{ token: string; user: User }> {
  const { signal, clear } = withTimeout(TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password, email: email ?? '' }),
      signal,
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      if (res.status === 409) throw new Error('El nombre de usuario ya está en uso');
      if (res.status === 400) throw new Error(body.trim() || 'Datos de registro inválidos');
      throw new Error('Error al registrar el usuario');
    }
    const data: AuthResponse = await res.json();
    return { token: data.token, user: { userId: data.userId, username: data.username, email: data.email, isAdmin: data.isAdmin ?? false } };
  } finally {
    clear();
  }
}

export async function login(username: string, password: string): Promise<{ token: string; user: User }> {
  const { signal, clear } = withTimeout(TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
      signal,
    });
    if (!res.ok) {
      if (res.status === 401) throw new Error('Usuario o contraseña incorrectos');
      throw new Error('Error al iniciar sesión');
    }
    const data: AuthResponse = await res.json();
    return { token: data.token, user: { userId: data.userId, username: data.username, email: data.email, isAdmin: data.isAdmin ?? false } };
  } finally {
    clear();
  }
}

export async function changePassword(currentPassword: string, newPassword: string, token: string): Promise<void> {
  const { signal, clear } = withTimeout(TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/auth/password`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ currentPassword, newPassword }),
      signal,
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      if (res.status === 401) throw new Error('La contraseña actual es incorrecta');
      if (res.status === 400) throw new Error(body.trim() || 'Datos inválidos');
      throw new Error('Error al cambiar la contraseña');
    }
  } finally {
    clear();
  }
}

export async function logout(token: string): Promise<void> {
  const { signal, clear } = withTimeout(TIMEOUT_MS);
  try {
    await fetch(`${API_BASE}/auth/logout`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
  } catch {
    // Ignore network errors — client-side token removal still proceeds.
  } finally {
    clear();
  }
}
