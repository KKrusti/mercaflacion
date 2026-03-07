import { useState, useEffect } from 'react';
import { getHousehold, createInvitation, leaveHousehold } from '../api/household';
import type { HouseholdMember } from '../api/household';

interface HouseholdSectionProps {
  token: string;
  currentUsername: string;
  onLeft: () => void;
}

function CopyIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="9" y="9" width="13" height="13" rx="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  );
}

export default function HouseholdSection({ token, currentUsername, onLeft }: HouseholdSectionProps) {
  const [members, setMembers] = useState<HouseholdMember[]>([]);
  const [inviteLink, setInviteLink] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getHousehold(token)
      .then((m) => { if (!cancelled) setMembers(m); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [token]);

  async function handleInvite() {
    setLoading(true);
    setError(null);
    setInviteLink(null);
    try {
      const inviteToken = await createInvitation(token);
      const link = `${window.location.origin}/?invite=${inviteToken}`;
      setInviteLink(link);
    } catch {
      setError('No se pudo crear la invitación');
    } finally {
      setLoading(false);
    }
  }

  async function handleLeave() {
    setLoading(true);
    setError(null);
    try {
      await leaveHousehold(token);
      onLeft();
    } catch {
      setError('No se pudo abandonar la unidad familiar');
    } finally {
      setLoading(false);
    }
  }

  function handleCopy() {
    if (!inviteLink) return;
    navigator.clipboard.writeText(inviteLink).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  const inHousehold = members.length > 0;

  return (
    <div className="household-section">
      <div className="household-section__title">Unidad familiar</div>

      {inHousehold && (
        <ul className="household-section__members" aria-label="Miembros de la unidad familiar">
          {members.map((m) => (
            <li key={m.id} className="household-section__member">
              {m.username}
              {m.username === currentUsername && (
                <span className="household-section__you"> (tú)</span>
              )}
            </li>
          ))}
        </ul>
      )}

      {inviteLink ? (
        <div className="household-section__invite-result">
          <div className="household-section__invite-row">
            <input
              readOnly
              value={inviteLink}
              className="household-section__invite-input"
              aria-label="Enlace de invitación"
              onFocus={(e) => e.target.select()}
            />
            <button
              type="button"
              className="household-section__copy-btn"
              onClick={handleCopy}
              aria-label="Copiar enlace"
            >
              {copied ? '✓' : <CopyIcon />}
            </button>
          </div>
          <p className="household-section__invite-note">Válido durante 24 horas</p>
        </div>
      ) : (
        <button
          type="button"
          className="user-menu__item"
          onClick={handleInvite}
          disabled={loading}
        >
          Invitar conviviente
        </button>
      )}

      {inHousehold && !inviteLink && (
        <button
          type="button"
          className="user-menu__item user-menu__item--danger"
          onClick={handleLeave}
          disabled={loading}
          aria-label="Abandonar unidad familiar"
        >
          Abandonar unidad
        </button>
      )}

      {error && <p className="household-section__error" role="alert">{error}</p>}
    </div>
  );
}
