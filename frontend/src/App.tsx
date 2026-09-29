import { useEffect, useState } from 'react';
import { LogOut, Shield, Monitor, Moon, Sun } from 'lucide-react';
import DeviceList from './components/DeviceList';
import Login from './components/Login';
import AdminPanel from './components/AdminPanel';
import { getCurrentUser, isPreviewMode, logout } from './services/api';
import type { SessionUser } from './services/types';

function App() {
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    const saved = window.localStorage.getItem('secure-switch-theme');
    return saved === 'light' || saved === 'dark' ? saved : window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  });
  const [currentUser, setCurrentUser] = useState<SessionUser | null>(null);
  const [showAdmin, setShowAdmin] = useState(() => isPreviewMode
    && new URLSearchParams(window.location.search).get('panel') === 'admin');
  const hidePreviewNotice = isPreviewMode && new URLSearchParams(window.location.search).has('capture');
  const [isInitializing, setIsInitializing] = useState(true);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    window.localStorage.setItem('secure-switch-theme', theme);
  }, [theme]);

  useEffect(() => {
    let cancelled = false;
    const restoreSession = async () => {
      try {
        const user = await getCurrentUser();
        if (!cancelled) setCurrentUser(user);
      } catch {
        if (!cancelled) setCurrentUser(null);
      } finally {
        if (!cancelled) setIsInitializing(false);
      }
    };
    void restoreSession();
    return () => { cancelled = true; };
  }, []);

  const handleLogout = async () => {
    if (isPreviewMode) {
      window.location.search = '';
      return;
    }
    try {
      await logout();
    } catch {
      // Clear the local session even if the server is unavailable.
    } finally {
      setCurrentUser(null);
      setShowAdmin(false);
    }
  };

  if (isInitializing) return <div className="app-shell" />;

  return (
    <div className="app-shell">
      {isPreviewMode && !hidePreviewNotice && <div className="preview-banner" role="status">Local preview · Example data only · Device actions are simulated</div>}
      <header className="site-header">
        <div className="header-inner">
          <div className="brand"><img className="brand-logo" src="/logo.svg" alt="" /><span>SecureSwitch</span></div>
          {currentUser && <nav className="main-nav" aria-label="Main navigation">
            <button className={`nav-link ${!showAdmin ? 'active' : ''}`} aria-current={!showAdmin ? 'page' : undefined} onClick={() => setShowAdmin(false)}><Monitor size={17} aria-hidden="true" /> Devices</button>
            {currentUser.is_admin && <button className={`nav-link ${showAdmin ? 'active' : ''}`} aria-current={showAdmin ? 'page' : undefined} onClick={() => setShowAdmin(true)}><Shield size={17} aria-hidden="true" /> Admin</button>}
          </nav>}
          <div className="header-actions"><button className="theme-toggle" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')} aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`} title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}>
            {theme === 'light' ? <Moon size={18} aria-hidden="true" /> : <Sun size={18} aria-hidden="true" />}
          </button>
          {currentUser && <button className="header-logout" onClick={handleLogout}><LogOut size={17} aria-hidden="true" /><span>Log out</span></button>}</div>
        </div>
      </header>
      <main className="page-content">{!currentUser ? <Login onLoginSuccess={setCurrentUser} /> : showAdmin && currentUser.is_admin ? <AdminPanel /> : <DeviceList />}</main>
    </div>
  );
}

export default App;
