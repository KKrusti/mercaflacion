import { useState, useRef, useEffect } from 'react';
import { Analytics as VercelAnalytics } from '@vercel/analytics/react';
import SearchBar from './components/SearchBar';
import ProductDetail from './components/ProductDetail';
import TicketUploader from './components/TicketUploader';
import Analytics from './components/Analytics';
import LoginModal from './components/LoginModal';
import ChangePasswordModal from './components/ChangePasswordModal';
import HouseholdSection from './components/HouseholdSection';
import AcceptInviteModal from './components/AcceptInviteModal';
import { logout } from './api/auth';
import { triggerEnrich, getEmailAccount } from './api/products';
import type { ProductBrowserState } from './components/ProductBrowser';
import type { AuthState } from './types';

const AUTH_STORAGE_KEY = 'mercaflacion_auth';

function loadAuth(): AuthState {
  try {
    const raw = localStorage.getItem(AUTH_STORAGE_KEY);
    if (raw) return JSON.parse(raw) as AuthState;
  } catch {
    // ignore corrupt storage
  }
  return { user: null, token: null };
}

function saveAuth(auth: AuthState) {
  if (auth.token) {
    localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(auth));
  } else {
    localStorage.removeItem(AUTH_STORAGE_KEY);
  }
}

type Tab = 'products' | 'analytics';

function AppLogo() {
  return (
    <img
      src="/logo.png"
      alt="Mercaflación"
      className="app-header__logo-img"
    />
  );
}

function UserIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="8" r="4" />
      <path d="M4 20c0-4 3.6-7 8-7s8 3 8 7" />
    </svg>
  );
}

function CheckSmallIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2.5"
      strokeLinecap="round" strokeLinejoin="round" width="14" height="14" aria-hidden="true">
      <polyline points="2 8 6 12 14 4" />
    </svg>
  );
}

function ChevronDownIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="user-menu__chevron">
      <polyline points="6 9 12 15 18 9" />
    </svg>
  );
}

function SunIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="4" />
      <line x1="12" y1="2"  x2="12" y2="4"  />
      <line x1="12" y1="20" x2="12" y2="22" />
      <line x1="4.22" y1="4.22"   x2="5.64" y2="5.64"   />
      <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
      <line x1="2"  y1="12" x2="4"  y2="12" />
      <line x1="20" y1="12" x2="22" y2="12" />
      <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
      <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  );
}

interface UserMenuProps {
  username: string;
  email?: string;
  token: string;
  isAdmin: boolean;
  onLogout: () => void;
  onChangePassword: () => void;
  onTriggerEnrich: () => void;
}

function UserMenu({ username, email, token, isAdmin, onLogout, onChangePassword, onTriggerEnrich }: UserMenuProps) {
  const [open, setOpen] = useState(false);
  const [enrichDone, setEnrichDone] = useState(false);
  const [linkedEmail, setLinkedEmail] = useState<string | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    getEmailAccount().then((acc) => {
      if (!cancelled) setLinkedEmail(acc?.emailAddress ?? null);
    }).catch(() => {});
    return () => { cancelled = true; };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    function handleClickOutside(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [open]);

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Escape') setOpen(false);
  }

  function handleEnrich() {
    setOpen(false);
    onTriggerEnrich();
    setEnrichDone(true);
    setTimeout(() => setEnrichDone(false), 3000);
  }

  return (
    <div className="user-menu" ref={rootRef} onKeyDown={handleKeyDown}>
      <button
        type="button"
        className="auth-btn auth-btn--active"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Menú de usuario: ${username}`}
      >
        <UserIcon />
        <span className="user-menu__name">{username}</span>
        <ChevronDownIcon />
      </button>
      {open && (
        <div className="user-menu__dropdown" role="menu">
          <div className="user-menu__info">
            <span className="user-menu__info-name">{username}</span>
            {email && <span className="user-menu__info-email">{email}</span>}
            {linkedEmail && (
              <span className="user-menu__info-linked-email">
                <span className="user-menu__info-linked-label">Correo asociado</span>
                {linkedEmail}
              </span>
            )}
          </div>
          <HouseholdSection
            token={token}
            currentUsername={username}
            onLeft={() => setOpen(false)}
          />
          <div className="user-menu__divider" />
          {isAdmin && (
            <button
              type="button"
              role="menuitem"
              className="user-menu__item"
              onClick={handleEnrich}
            >
              {enrichDone ? <><CheckSmallIcon /> Lanzado</> : 'Enriquecer imágenes'}
            </button>
          )}
          <button
            type="button"
            role="menuitem"
            className="user-menu__item"
            onClick={() => { setOpen(false); onChangePassword(); }}
          >
            Cambiar contraseña
          </button>
          <button
            type="button"
            role="menuitem"
            className="user-menu__item user-menu__item--danger"
            onClick={() => { setOpen(false); onLogout(); }}
          >
            Cerrar sesión
          </button>
        </div>
      )}
    </div>
  );
}

export default function App() {
  const [selectedProductId, setSelectedProductId] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('products');
  const [browserState, setBrowserState] = useState<ProductBrowserState>({
    page: 0,
    pageSize: 48,
    columns: 3,
  });
  const [auth, setAuth] = useState<AuthState>(loadAuth);
  const [showLoginModal, setShowLoginModal] = useState(false);
  const [showChangePasswordModal, setShowChangePasswordModal] = useState(false);
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    const stored = localStorage.getItem('mercaflacion_theme');
    if (stored === 'light' || stored === 'dark') return stored;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  });

  function toggleTheme() {
    const next = theme === 'light' ? 'dark' : 'light';
    setTheme(next);
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('mercaflacion_theme', next);
  }
  const [pendingInviteToken, setPendingInviteToken] = useState<string | null>(
    () => new URLSearchParams(window.location.search).get('invite'),
  );

  // Apply the resolved theme to <html> on first render.
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // When the page loads with an invite link and the user is not logged in,
  // open the login modal automatically so they know what to do.
  useEffect(() => {
    if (pendingInviteToken && !auth.user) {
      setShowLoginModal(true);
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function handleSelectProduct(id: string) {
    setSelectedProductId(id);
  }

  function handleBack() {
    setSelectedProductId(null);
  }

  function handleLogoClick() {
    setSelectedProductId(null);
    setActiveTab('products');
    setBrowserState((prev) => ({ ...prev, page: 0 }));
  }

  function handleAuth(newAuth: AuthState) {
    setAuth(newAuth);
    saveAuth(newAuth);
    setShowLoginModal(false);
  }

  async function handleLogout() {
    // Revoke the token server-side before clearing local state so that a
    // stolen token cannot be reused within its remaining TTL.
    if (auth.token) {
      await logout(auth.token).catch(() => {});
    }
    const cleared: AuthState = { user: null, token: null };
    setAuth(cleared);
    saveAuth(cleared);
  }

  function handleTriggerEnrich() {
    if (auth.token) {
      triggerEnrich(auth.token).catch(() => {});
    }
  }

  return (
    <>
    <div className="app">
      <header className="app-header">
        <button
          className="app-header__logo"
          onClick={handleLogoClick}
          aria-label="Ir a la página principal"
        >
          <AppLogo />
        </button>
        <p className="app-header__subtitle">
          Consulta y compara el historial de precios de tus productos favoritos
        </p>
        <div className="app-header__actions">
          <button
            type="button"
            className="theme-toggle"
            onClick={toggleTheme}
            aria-label={theme === 'light' ? 'Activar modo oscuro' : 'Activar modo claro'}
          >
            {theme === 'light' ? <MoonIcon /> : <SunIcon />}
          </button>
          {auth.user ? (
            <UserMenu
              username={auth.user.username}
              email={auth.user.email}
              token={auth.token ?? ''}
              isAdmin={auth.user.isAdmin}
              onLogout={() => { void handleLogout(); }}
              onChangePassword={() => setShowChangePasswordModal(true)}
              onTriggerEnrich={handleTriggerEnrich}
            />
          ) : (
            <button
              className="auth-btn"
              onClick={() => setShowLoginModal(true)}
              aria-label="Iniciar sesión"
            >
              <UserIcon />
              Entrar
            </button>
          )}
          {auth.user && <TicketUploader />}
        </div>
      </header>

      {showLoginModal && (
        <LoginModal
          onAuth={handleAuth}
          onClose={() => setShowLoginModal(false)}
          hint={pendingInviteToken ? 'Has recibido una invitación para unirte a una unidad familiar. Inicia sesión o crea una cuenta para aceptarla.' : undefined}
        />
      )}

      {showChangePasswordModal && auth.token && (
        <ChangePasswordModal
          token={auth.token}
          onClose={() => setShowChangePasswordModal(false)}
        />
      )}

      {pendingInviteToken && auth.token && (
        <AcceptInviteModal
          inviteToken={pendingInviteToken}
          authToken={auth.token}
          onClose={() => setPendingInviteToken(null)}
        />
      )}

      {!auth.user ? (
        <div className="app-content guest-screen">
          <div className="guest-screen__card">
            <p className="guest-screen__message">
              Inicia sesión para consultar y comparar el historial de precios de tus productos.
            </p>
            <button
              className="auth-btn guest-screen__btn"
              onClick={() => setShowLoginModal(true)}
            >
              <UserIcon />
              Iniciar sesión
            </button>
          </div>
        </div>
      ) : selectedProductId ? (
        <div className="app-content">
          <ProductDetail
            productId={selectedProductId}
            onBack={handleBack}
            token={auth.token}
          />
        </div>
      ) : (
        <>
          <nav className="app-tabs" role="tablist" aria-label="Secciones de la aplicación">
            <button
              role="tab"
              aria-selected={activeTab === 'products'}
              aria-controls="tab-panel-products"
              id="tab-products"
              className={`app-tabs__tab${activeTab === 'products' ? ' app-tabs__tab--active' : ''}`}
              onClick={() => setActiveTab('products')}
            >
              Productos
            </button>
            <button
              role="tab"
              aria-selected={activeTab === 'analytics'}
              aria-controls="tab-panel-analytics"
              id="tab-analytics"
              className={`app-tabs__tab${activeTab === 'analytics' ? ' app-tabs__tab--active' : ''}`}
              onClick={() => setActiveTab('analytics')}
            >
              Analítica
            </button>
          </nav>

          <div className="app-content">
            <div
              role="tabpanel"
              id="tab-panel-products"
              aria-labelledby="tab-products"
              hidden={activeTab !== 'products'}
            >
              {activeTab === 'products' && (
                <SearchBar
                  onSelectProduct={handleSelectProduct}
                  browserState={browserState}
                  onBrowserStateChange={setBrowserState}
                />
              )}
            </div>
            <div
              role="tabpanel"
              id="tab-panel-analytics"
              aria-labelledby="tab-analytics"
              hidden={activeTab !== 'analytics'}
            >
              {activeTab === 'analytics' && (
                <Analytics onSelectProduct={handleSelectProduct} />
              )}
            </div>
          </div>
        </>
      )}
    </div>
    <VercelAnalytics />
    </>
  );
}
