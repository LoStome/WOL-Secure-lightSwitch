import { useEffect, useState } from 'react';
import { Power } from 'lucide-react';
import { wakeHost, shutdownHost } from '../services/api';
import type { Host } from '../services/types';
import { DEVICE_ACTION_TIMEOUT_MS, DeviceActionTracker } from './deviceActionState';
import type { DeviceActionState } from './deviceActionState';

interface DeviceCardProps { host: Host }

const DeviceCard = ({ host }: DeviceCardProps) => {
  const [actionState, setActionState] = useState<DeviceActionState>({ status: 'idle' });
  const [actionTracker] = useState(() => new DeviceActionTracker(setActionState));
  const isOn = host.online;
  const isLoading = actionState.status === 'sending' || actionState.status === 'waiting';

  useEffect(() => {
    if (actionState.status === 'sending' || actionState.status === 'waiting' || actionState.status === 'timedOut') actionTracker.confirm(isOn);
  }, [actionState, actionTracker, isOn]);
  useEffect(() => () => actionTracker.dispose(), [actionTracker]);

  const handlePowerToggle = async () => {
    actionTracker.start(!isOn);
    try {
      if (isOn) await shutdownHost(host.ID);
      else await wakeHost(host.ID);
      actionTracker.commandSent();
    } catch (error) {
      console.error('Action failed:', error);
      actionTracker.fail();
    }
  };

  const statusMessage = actionState.status === 'sending' ? 'Sending command…'
    : actionState.status === 'waiting' ? 'Command sent. Checking device status…'
      : actionState.status === 'timedOut' ? `No confirmation after ${DEVICE_ACTION_TIMEOUT_MS / 1000} seconds. You can try again.`
        : actionState.status === 'error' ? 'Action failed. Please try again.' : '';

  return <article className="device-card">
    <div className="device-main">
      <div className="device-heading"><div><h2>{host.Name}</h2><span className={`status-label ${isOn ? 'online' : 'offline'}`}><span className="status-dot" />{isOn ? 'Online' : 'Offline'}</span></div><button className={`device-power ${isOn ? 'is-on' : 'is-off'}`} onClick={handlePowerToggle} disabled={isLoading} aria-busy={isLoading} aria-label={`${isOn ? 'Turn off' : 'Turn on'} ${host.Name}`} title={`${isOn ? 'Turn off' : 'Turn on'} ${host.Name}`}><Power size={23} aria-hidden="true" /></button></div>
      <div className="device-details"><div><span>IP address</span><code>{host.IP}</code></div><div><span>MAC address</span><code>{host.MAC}</code></div><div><span>Last ping</span><code>{host.last_pinged && host.last_pinged !== 'N/A' ? host.last_pinged : 'Not yet checked'}</code></div></div>
      {statusMessage && <p className="action-message" role={actionState.status === 'error' || actionState.status === 'timedOut' ? 'alert' : 'status'}>{statusMessage}</p>}
    </div>
  </article>;
};

export default DeviceCard;
