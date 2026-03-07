import { useState, useRef, useEffect } from 'react';
import SearchBar from './components/SearchBar';
import ProductDetail from './components/ProductDetail';
import TicketUploader from './components/TicketUploader';
import Analytics from './components/Analytics';
import LoginModal from './components/LoginModal';
import ChangePasswordModal from './components/ChangePasswordModal';
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
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 400 100"
      aria-label="Mercaflación"
      role="img"
      className="app-header__logo-svg"
    >
      <rect x="50" y="10" width="300" height="80" rx="40" ry="40" fill="#00A859" stroke="#FFFFFF" strokeWidth="4"/>
      <text x="200" y="60" fontFamily="Arial, sans-serif" fontSize="32" fontWeight="bold" fill="#FFFFFF" textAnchor="middle" letterSpacing="1">MERCAFLACION</text>
    </svg>
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

function ChevronDownIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="user-menu__chevron">
      <polyline points="6 9 12 15 18 9" />
    </svg>
  );
}

interface UserMenuProps {
  username: string;
  email?: string;
  onLogout: () => void;
  onChangePassword: () => void;
}

function UserMenu({ username, email, onLogout, onChangePassword }: UserMenuProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

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
          </div>
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

  function handleLogout() {
    const cleared: AuthState = { user: null, token: null };
    setAuth(cleared);
    saveAuth(cleared);
  }

  return (
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
          {auth.user ? (
            <UserMenu
              username={auth.user.username}
              email={auth.user.email}
              onLogout={handleLogout}
              onChangePassword={() => setShowChangePasswordModal(true)}
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
        <LoginModal onAuth={handleAuth} onClose={() => setShowLoginModal(false)} />
      )}

      {showChangePasswordModal && auth.token && (
        <ChangePasswordModal
          token={auth.token}
          onClose={() => setShowChangePasswordModal(false)}
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
  );
}
