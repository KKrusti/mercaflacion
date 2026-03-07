const API_BASE = '/api';
const TIMEOUT_MS = 10_000;

function withTimeout(ms: number): { signal: AbortSignal; clear: () => void } {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), ms);
  return { signal: controller.signal, clear: () => clearTimeout(timer) };
}

function authHeaders(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}

export interface HouseholdMember {
  id: number;
  username: string;
}

export async function getHousehold(token: string): Promise<HouseholdMember[]> {
  const { signal, clear } = withTimeout(TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/household`, { headers: authHeaders(token), signal });
    if (!res.ok) throw new Error('Error al obtener la unidad familiar');
    const data = await res.json() as { members: HouseholdMember[] };
    return data.members ?? [];
  } finally {
    clear();
  }
}

export async function createInvitation(token: string): Promise<string> {
  const { signal, clear } = withTimeout(TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/household/invite`, {
      method: 'POST',
      headers: authHeaders(token),
      signal,
    });
    if (!res.ok) throw new Error('Error al crear la invitación');
    const data = await res.json() as { token: string };
    return data.token;
  } finally {
    clear();
  }
}

export async function acceptInvitation(inviteToken: string, authToken: string): Promise<void> {
  const { signal, clear } = withTimeout(TIMEOUT_MS);
  try {
    const res = await fetch(
      `${API_BASE}/household/accept?token=${encodeURIComponent(inviteToken)}`,
      { method: 'POST', headers: authHeaders(authToken), signal },
    );
    if (!res.ok) {
      if (res.status === 404) throw new Error('La invitación no existe o ha expirado');
      if (res.status === 400) throw new Error('Invitación inválida');
      throw new Error('Error al unirse a la unidad familiar');
    }
  } finally {
    clear();
  }
}

export async function leaveHousehold(token: string): Promise<void> {
  const { signal, clear } = withTimeout(TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/household`, {
      method: 'DELETE',
      headers: authHeaders(token),
      signal,
    });
    if (!res.ok) throw new Error('Error al abandonar la unidad familiar');
  } finally {
    clear();
  }
}
