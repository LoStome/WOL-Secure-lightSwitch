import React, { useEffect, useState } from 'react';
import { Power } from 'lucide-react';
import { wakeHost, shutdownHost } from '../services/api';
import type { Host } from '../services/api';
import { DEVICE_ACTION_TIMEOUT_MS, DeviceActionTracker } from './deviceActionState';
import type { DeviceActionState } from './deviceActionState';

interface DeviceCardProps {
  host: Host;
}

const DeviceCard: React.FC<DeviceCardProps> = ({ host }) => {
  const [actionState, setActionState] = useState<DeviceActionState>({ status: 'idle' });
  const [actionTracker] = useState(() => new DeviceActionTracker(setActionState));

  // Use the backend's real-time state for UI
  const isOn = host.online;
  const isLoading = actionState.status === 'sending' || actionState.status === 'waiting';

  useEffect(() => {
    if (actionState.status === 'sending' || actionState.status === 'waiting' || actionState.status === 'timedOut') {
      actionTracker.confirm(isOn);
    }
  }, [actionState, actionTracker, isOn]);

  useEffect(() => () => actionTracker.dispose(), [actionTracker]);

  const handlePowerToggle = async () => {
    const expectedState = !isOn;
    actionTracker.start(expectedState);
    try {
      if (isOn) {
        // Currently ON, pressing it means Turn OFF (Shutdown)
        await shutdownHost(host.ID);
      } else {
        // Currently OFF, pressing it means Turn ON (Wake)
        await wakeHost(host.ID);
      }
      actionTracker.commandSent();
    } catch (error) {
      console.error('Action failed:', error);
      actionTracker.fail();
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

      <div className="flex flex-col items-center gap-3">
        <button
          onClick={handlePowerToggle}
          disabled={isLoading}
          aria-busy={isLoading}
          aria-label={`${isOn ? 'Turn off' : 'Turn on'} ${host.Name}`}
          className={`relative flex items-center justify-center w-16 h-16 rounded-full transition-all duration-300 shadow-inner overflow-hidden ${buttonStyle}`}
        >
          {/* Glow effect */}
          <div className={`absolute inset-0 rounded-full blur-md opacity-50 ${glowStyle}`}></div>

          <Power aria-hidden="true" className={`w-8 h-8 z-10 ${isLoading ? 'animate-pulse' : ''}`} />
        </button>

        {actionState.status !== 'idle' && (
          <p
            className={`max-w-64 text-sm text-center ${actionState.status === 'error' || actionState.status === 'timedOut' ? 'text-amber-400' : 'text-zinc-400'}`}
            role={actionState.status === 'error' || actionState.status === 'timedOut' ? 'alert' : 'status'}
            aria-live={actionState.status === 'error' || actionState.status === 'timedOut' ? 'assertive' : 'polite'}
          >
            {actionState.status === 'sending' && 'Sending power command...'}
            {actionState.status === 'waiting' && 'Command sent. Waiting for device confirmation...'}
            {actionState.status === 'timedOut' && `No confirmation after ${DEVICE_ACTION_TIMEOUT_MS / 1000} seconds. You can try again.`}
            {actionState.status === 'error' && 'Action failed. Please try again.'}
          </p>
        )}
      </div>
    </div>
  );
};

export default DeviceCard;
