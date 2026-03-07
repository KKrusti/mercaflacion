import { useState } from 'react';
import { changePassword } from '../api/auth';

interface ChangePasswordModalProps {
  token: string;
  onClose: () => void;
}

export default function ChangePasswordModal({ token, onClose }: ChangePasswordModalProps) {
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await changePassword(currentPassword, newPassword, token);
      setSuccess(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error inesperado');
    } finally {
      setLoading(false);
    }
  }

  function handleOverlayClick(e: React.MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-label="Cambiar contraseña" onClick={handleOverlayClick}>
      <div className="modal">
        <div className="modal__header">
          <h2 className="modal__title">Cambiar contraseña</h2>
          <button className="modal__close" aria-label="Cerrar" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        {success ? (
          <div className="modal__form">
            <p className="modal__success" role="status">Contraseña actualizada correctamente.</p>
            <button className="modal__submit" type="button" onClick={onClose}>
              Cerrar
            </button>
          </div>
        ) : (
          <form className="modal__form" onSubmit={handleSubmit} noValidate>
            <div className="modal__field">
              <label className="modal__label" htmlFor="cp-current">Contraseña actual</label>
              <input
                id="cp-current"
                className="modal__input"
                type="password"
                autoComplete="current-password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                required
                disabled={loading}
                placeholder="Tu contraseña actual"
              />
            </div>

            <div className="modal__field">
              <label className="modal__label" htmlFor="cp-new">Nueva contraseña</label>
              <input
                id="cp-new"
                className="modal__input"
                type="password"
                autoComplete="new-password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
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
              {loading ? 'Guardando...' : 'Actualizar contraseña'}
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
