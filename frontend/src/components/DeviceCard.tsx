import React, { useState } from 'react';
import { Power } from 'lucide-react';
import { wakeHost, shutdownHost } from '../services/api';
import type { Host } from '../services/api';

interface DeviceCardProps {
  host: Host;
}

const DeviceCard: React.FC<DeviceCardProps> = ({ host }) => {
  const [isLoading, setIsLoading] = useState<boolean>(false);

  // Track the expected state after clicking (true for ON, false for OFF). If null, no action is pending.
  const [expectedState, setExpectedState] = useState<boolean | null>(null);

  // Use the backend's real-time state for UI
  const isOn = host.online;

  // Clear loading state and expected state once the backend state matches what we asked for
  React.useEffect(() => {
    if (expectedState !== null && isOn === expectedState) {
      setIsLoading(false);
      setExpectedState(null);
    }
  }, [isOn, expectedState]);

  const handlePowerToggle = async () => {
    setIsLoading(true);
    try {
      if (isOn) {
        // Currently ON, pressing it means Turn OFF (Shutdown)
        setExpectedState(false);
        await shutdownHost(host.ID);
      } else {
        // Currently OFF, pressing it means Turn ON (Wake)
        setExpectedState(true);
        await wakeHost(host.ID);
      }
    } catch (error) {
      console.error('Action failed:', error);
      setIsLoading(false);
      setExpectedState(null);
    }
  };

  // Determine button styles based on state
  let buttonStyle = '';
  let glowStyle = '';
  
  if (isLoading) {
    // Gray state while loading/waiting for ping confirm
    buttonStyle = 'bg-zinc-500/10 text-zinc-400 cursor-wait shadow-none';
    glowStyle = 'bg-zinc-500/10';
  } else if (isOn) {
    // Green state
    buttonStyle = 'bg-emerald-500/10 text-emerald-400 hover:bg-emerald-500/20 hover:shadow-[0_0_20px_rgba(16,185,129,0.4)] cursor-pointer';
    glowStyle = 'bg-emerald-500/30';
  } else {
    // Red state
    buttonStyle = 'bg-rose-500/10 text-rose-400 hover:bg-rose-500/20 hover:shadow-[0_0_20px_rgba(244,63,94,0.4)] cursor-pointer';
    glowStyle = 'bg-rose-500/30';
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-2xl p-6 flex flex-col md:flex-row items-center justify-between shadow-xl shadow-black/50 hover:border-zinc-700 transition-all group gap-6">
      <div className="flex flex-col flex-1 space-y-1 text-center md:text-left">
        <h3 className="text-xl font-semibold text-zinc-100 group-hover:text-white">{host.Name}</h3>
        <p className="text-sm font-mono text-zinc-400">{host.IP}</p>
        <p className="text-xs font-mono text-zinc-500 uppercase tracking-widest">{host.MAC}</p>
        {host.last_pinged && (
          <p className="text-xs text-zinc-400 mt-2">
            Last pinged: <span className="font-mono text-zinc-300">{host.last_pinged}</span>
          </p>
        )}
      </div>

      <button
        onClick={handlePowerToggle}
        disabled={isLoading}
        className={`relative flex items-center justify-center w-16 h-16 rounded-full transition-all duration-300 shadow-inner overflow-hidden ${buttonStyle}`}
      >
        {/* Glow effect */}
        <div className={`absolute inset-0 rounded-full blur-md opacity-50 ${glowStyle}`}></div>
        
        <Power className={`w-8 h-8 z-10 ${isLoading ? 'animate-pulse' : ''}`} />
      </button>
    </div>
  );
};

export default DeviceCard;
