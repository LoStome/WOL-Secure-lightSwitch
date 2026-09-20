import { useState, useEffect } from 'react';
import DeviceList from './components/DeviceList';
import Login from './components/Login';
import AdminPanel from './components/AdminPanel'; 
import { Zap, LogOut, Shield } from 'lucide-react';
import { getCurrentUser, logout, type User } from './services/api';

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [showAdmin, setShowAdmin] = useState(false);
  const [isInitializing, setIsInitializing] = useState(true);

  useEffect(() => {
    let cancelled = false;

    const restoreSession = async () => {
      try {
        const user = await getCurrentUser();
        if (!cancelled) {
          setCurrentUser(user);
          setIsAuthenticated(true);
        }
      } catch {
        if (!cancelled) {
          setCurrentUser(null);
          setIsAuthenticated(false);
        }
      } finally {
        if (!cancelled) {
          setIsInitializing(false);
        }
      }
    };

    void restoreSession();
    return () => {
      cancelled = true;
    };
  }, []);

  const handleLoginSuccess = (user: User) => {
    setCurrentUser(user);
    setIsAuthenticated(true);
  };

  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      // Local logout must still succeed if the session is already invalid or unreachable.
    } finally {
      setCurrentUser(null);
      setIsAuthenticated(false);
      setShowAdmin(false);
    }
  };

  if (isInitializing) {
    return <div className="min-h-screen bg-zinc-950" />;
  }

  return (
    <div className="min-h-screen pt-12 pb-24 px-4 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-zinc-900 via-zinc-950 to-black">
      <header className="max-w-7xl mx-auto mb-16 text-center relative">
        {isAuthenticated && (
          <div className="flex justify-end gap-2 mb-6 md:mb-0 md:absolute md:top-0 md:right-0">
            {currentUser?.is_admin && !showAdmin && (
              <button 
                onClick={() => setShowAdmin(true)}
                className="flex items-center gap-2 px-3 py-1.5 bg-zinc-800/50 hover:bg-zinc-800 font-mono text-zinc-300 text-sm rounded-lg transition-colors border border-zinc-700/50"
              >
                <Shield className="w-4 h-4" />
                Admin
              </button>
            )}
            {showAdmin && (
              <button 
                onClick={() => setShowAdmin(false)}
                className="flex items-center gap-2 px-3 py-1.5 bg-zinc-800/50 hover:bg-zinc-800 font-mono text-zinc-300 text-sm rounded-lg transition-colors border border-zinc-700/50"
              >
                <Zap className="w-4 h-4" />
                Devices
              </button>
            )}
            <button 
              onClick={handleLogout}
              className="flex items-center gap-2 px-3 py-1.5 bg-red-900/20 hover:bg-red-900/40 text-red-400 font-mono text-sm rounded-lg transition-colors border border-red-900/30"
            >
              <LogOut className="w-4 h-4" />
              Logout
            </button>
          </div>
        )}

        <div className="flex items-center justify-center gap-3 mb-4 mt-8 md:mt-0">
          <div className="bg-blue-500/10 p-3 rounded-2xl ring-1 ring-blue-500/50 shadow-[0_0_30px_rgba(59,130,246,0.2)]">
            <Zap className="w-8 h-8 text-blue-400" />
          </div>
          <h1 className="text-4xl md:text-5xl font-bold tracking-tight bg-clip-text text-transparent bg-gradient-to-r from-zinc-100 to-zinc-500">
            SecureSwitch
          </h1>
        </div>
        <p className="text-zinc-500 font-mono text-sm tracking-widest mt-2 uppercase">WOL / Remote Shutdown</p>
      </header>

      <main className="w-full">
        {!isAuthenticated ? (
          <Login onLoginSuccess={handleLoginSuccess} />
        ) : showAdmin && currentUser?.is_admin ? (
          <AdminPanel />
        ) : (
          <DeviceList />
        )}
      </main>

      <footer className="fixed bottom-0 left-0 right-0 p-4 text-center text-zinc-600 text-xs font-mono backdrop-blur-md bg-zinc-950/80 border-t border-zinc-900">
        &copy; {new Date().getFullYear()} WOL-Secure-lightSwitch
      </footer>
    </div>
  );
}

export default App;
