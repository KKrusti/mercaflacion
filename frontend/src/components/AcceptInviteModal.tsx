import { useState } from 'react';
import { acceptInvitation } from '../api/household';

interface AcceptInviteModalProps {
  inviteToken: string;
  authToken: string;
  onClose: () => void;
}

export default function AcceptInviteModal({ inviteToken, authToken, onClose }: AcceptInviteModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleAccept() {
    setLoading(true);
    setError(null);
    try {
      await acceptInvitation(inviteToken, authToken);
      // Reload so that shared products are fetched with the new household scope.
      window.history.replaceState({}, '', '/');
      window.location.reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error al unirse a la unidad familiar');
      setLoading(false);
    }
  }

  function handleOverlayClick(e: React.MouseEvent<HTMLDivElement>) {
    if (e.target === e.currentTarget) onClose();
  }

  return (
    <div className="modal" role="dialog" aria-modal="true" aria-labelledby="accept-invite-title" onClick={handleOverlayClick}>
      <div className="modal__card">
        <div className="modal__header">
          <h2 className="modal__title" id="accept-invite-title">Unidad familiar</h2>
          <button type="button" className="modal__close" onClick={onClose} aria-label="Cerrar">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" aria-hidden="true">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <div className="modal__form">
          <p>Has recibido una invitación para unirte a una unidad familiar.</p>
          <p>Si aceptas, compartirás las compras y el historial de precios con los otros miembros.</p>

          {error && <p className="modal__error" role="alert">{error}</p>}

          <div className="modal__actions">
            <button
              type="button"
              className="btn btn--secondary"
              onClick={onClose}
              disabled={loading}
            >
              Cancelar
            </button>
            <button
              type="button"
              className="btn btn--primary"
              onClick={handleAccept}
              disabled={loading}
            >
              {loading ? 'Uniéndose…' : 'Aceptar invitación'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
