import React, { useState } from 'react';
import { Power } from 'lucide-react';
import { wakeHost, shutdownHost } from '../services/api';
import type { Host } from '../services/api';

interface DeviceCardProps {
  host: Host;
}

const DeviceCard: React.FC<DeviceCardProps> = ({ host }) => {
  // Simple state to track the UI visually (green for ON ping, red for OFF shutdown ping)
  // Later this will be driven by the ping API.
  const [isOn, setIsOn] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(false);

  const handlePowerToggle = async () => {
    setIsLoading(true);
    try {
      if (isOn) {
        // Currently ON, pressing it means Turn OFF (Shutdown)
        await shutdownHost(host.ID);
        setIsOn(false);
      } else {
        // Currently OFF, pressing it means Turn ON (Wake)
        await wakeHost(host.ID);
        setIsOn(true);
      }
    } catch (error) {
      console.error('Action failed:', error);
      // Optionally show a toast or alert here
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-2xl p-6 flex flex-col md:flex-row items-center justify-between shadow-xl shadow-black/50 hover:border-zinc-700 transition-all group gap-6">
      <div className="flex flex-col flex-1 space-y-1 text-center md:text-left">
        <h3 className="text-xl font-semibold text-zinc-100 group-hover:text-white">{host.Name}</h3>
        <p className="text-sm font-mono text-zinc-400">{host.IP}</p>
        <p className="text-xs font-mono text-zinc-500 uppercase tracking-widest">{host.MAC}</p>
      </div>

      <button
        onClick={handlePowerToggle}
        disabled={isLoading}
        className={`relative flex items-center justify-center w-16 h-16 rounded-full transition-all duration-300 shadow-inner overflow-hidden ${
          isOn 
            ? 'bg-emerald-500/10 text-emerald-400 hover:bg-emerald-500/20 hover:shadow-[0_0_20px_rgba(16,185,129,0.4)]' 
            : 'bg-rose-500/10 text-rose-400 hover:bg-rose-500/20 hover:shadow-[0_0_20px_rgba(244,63,94,0.4)]'
        } ${isLoading ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
      >
        {/* Glow effect */}
        <div className={`absolute inset-0 rounded-full blur-md opacity-50 ${isOn ? 'bg-emerald-500/30' : 'bg-rose-500/30'}`}></div>
        
        <Power className={`w-8 h-8 z-10 ${isLoading ? 'animate-pulse' : ''}`} />
      </button>
    </div>
  );
};

export default DeviceCard;
