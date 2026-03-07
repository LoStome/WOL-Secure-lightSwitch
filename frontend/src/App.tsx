import DeviceList from './components/DeviceList';
import { Zap } from 'lucide-react';

function App() {
  return (
    <div className="min-h-screen pt-12 pb-24 px-4 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-zinc-900 via-zinc-950 to-black">
      <header className="max-w-7xl mx-auto mb-16 text-center">
        <div className="flex items-center justify-center gap-3 mb-4">
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
        <DeviceList />
      </main>

      <footer className="fixed bottom-0 left-0 right-0 p-4 text-center text-zinc-600 text-xs font-mono backdrop-blur-md bg-zinc-950/80 border-t border-zinc-900">
        &copy; {new Date().getFullYear()} WOL-Secure-lightSwitch
      </footer>
    </div>
  );
}

export default App;
