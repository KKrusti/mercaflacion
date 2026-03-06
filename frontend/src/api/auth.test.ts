import { describe, it, expect, vi, beforeEach } from 'vitest';
import { register, login, logout } from './auth';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

function makeResponse(status: number, body: unknown): Response {
  const isString = typeof body === 'string';
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(isString ? body : JSON.stringify(body)),
  } as unknown as Response;
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('register', () => {
  it('returns token and user on success', async () => {
    mockFetch.mockResolvedValue(
      makeResponse(201, { token: 'tok123', userId: 1, username: 'carlos' }),
    );
    const result = await register('carlos', 'securepassword');
    expect(result.token).toBe('tok123');
    expect(result.user).toEqual({ userId: 1, username: 'carlos' });
  });

  it('throws a localized error when the username already exists (409)', async () => {
    mockFetch.mockResolvedValue(makeResponse(409, 'Conflict'));
    await expect(register('carlos', 'securepassword')).rejects.toThrow(
      'El nombre de usuario ya está en uso',
    );
  });

  it('throws a localized error when the data is invalid (400)', async () => {
    mockFetch.mockResolvedValue(makeResponse(400, 'Bad request: username must be at least 3 characters'));
    await expect(register('ab', 'securepassword')).rejects.toThrow(
      'Bad request: username must be at least 3 characters',
    );
  });

  it('throws a generic error on server error (500)', async () => {
    mockFetch.mockResolvedValue(makeResponse(500, 'Internal server error'));
    await expect(register('carlos', 'securepassword')).rejects.toThrow('Error al registrar el usuario');
  });

  it('calls /api/auth/register with POST method and correct body', async () => {
    mockFetch.mockResolvedValue(
      makeResponse(201, { token: 'tok', userId: 2, username: 'user' }),
    );
    await register('user', 'password123');
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/auth/register',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ username: 'user', password: 'password123' }),
      }),
    );
  });
});

describe('login', () => {
  it('returns token and user on success', async () => {
    mockFetch.mockResolvedValue(
      makeResponse(200, { token: 'tok456', userId: 5, username: 'maria' }),
    );
    const result = await login('maria', 'mypassword');
    expect(result.token).toBe('tok456');
    expect(result.user).toEqual({ userId: 5, username: 'maria' });
  });

  it('throws a localized error when credentials are invalid (401)', async () => {
    mockFetch.mockResolvedValue(makeResponse(401, 'Unauthorized'));
    await expect(login('user', 'wrong')).rejects.toThrow('Usuario o contraseña incorrectos');
  });

  it('throws a generic error on server error (500)', async () => {
    mockFetch.mockResolvedValue(makeResponse(500, 'Internal server error'));
    await expect(login('user', 'pass')).rejects.toThrow('Error al iniciar sesión');
  });

  it('calls /api/auth/login with POST method and correct body', async () => {
    mockFetch.mockResolvedValue(
      makeResponse(200, { token: 'tok', userId: 1, username: 'u' }),
    );
    await login('u', 'p');
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ username: 'u', password: 'p' }),
      }),
    );
  });
});

describe('logout', () => {
  it('does not throw any error', () => {
    expect(() => logout()).not.toThrow();
  });
});
