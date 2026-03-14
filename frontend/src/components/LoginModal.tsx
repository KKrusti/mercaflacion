import { useState } from 'react';
import { register, login } from '../api/auth';
import type { AuthState } from '../types';

interface LoginModalProps {
  onAuth: (auth: AuthState) => void;
  onClose: () => void;
  hint?: string;
}

type Mode = 'login' | 'register';

export default function LoginModal({ onAuth, onClose, hint }: LoginModalProps) {
  const [mode, setMode] = useState<Mode>('login');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const result = mode === 'login'
        ? await login(username, password)
        : await register(username, password, email);
      onAuth({ token: result.token, user: result.user });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error inesperado');
    } finally {
      setLoading(false);
    }
  }

  function handleOverlayClick(e: React.MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }

  function switchMode(next: Mode) {
    setMode(next);
    setError(null);
    setEmail('');
  }

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-label={mode === 'login' ? 'Iniciar sesión' : 'Registrarse'} onClick={handleOverlayClick}>
      <div className="modal">
        <div className="modal__header">
          <h2 className="modal__title">
            {mode === 'login' ? 'Iniciar sesión' : 'Crear cuenta'}
          </h2>
          <button className="modal__close" aria-label="Cerrar" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <div className="modal__tabs">
          <button
            className={`modal__tab${mode === 'login' ? ' modal__tab--active' : ''}`}
            onClick={() => switchMode('login')}
            type="button"
          >
            Entrar
          </button>
          <button
            className={`modal__tab${mode === 'register' ? ' modal__tab--active' : ''}`}
            onClick={() => switchMode('register')}
            type="button"
          >
            Registrarse
          </button>
        </div>

        {hint && <p className="modal__hint">{hint}</p>}

        <form className="modal__form" onSubmit={handleSubmit} noValidate>
          <div className="modal__field">
            <label className="modal__label" htmlFor="auth-username">Usuario</label>
            <input
              id="auth-username"
              className="modal__input"
              type="text"
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              minLength={3}
              disabled={loading}
              placeholder="Nombre de usuario"
            />
          </div>

          {mode === 'register' && (
            <div className="modal__field">
              <label className="modal__label" htmlFor="auth-email">Correo electrónico <span className="modal__optional">(opcional)</span></label>
              <input
                id="auth-email"
                className="modal__input"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                disabled={loading}
                placeholder="tu@correo.com"
              />
            </div>
          )}

          <div className="modal__field">
            <label className="modal__label" htmlFor="auth-password">Contraseña</label>
            <input
              id="auth-password"
              className="modal__input"
              type="password"
              autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              disabled={loading}
              placeholder="Mínimo 8 caracteres"
            />
          </div>

          {error && (
            <p className="modal__error" role="alert">{error}</p>
          )}

          <button className="modal__submit" type="submit" disabled={loading}>
            {loading
              ? 'Cargando...'
              : mode === 'login' ? 'Entrar' : 'Crear cuenta'}
          </button>
        </form>
      </div>
    </div>
  );
}
