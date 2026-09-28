import { useEffect, useState } from 'react';
import { CircleAlert, MonitorOff } from 'lucide-react';
import DeviceCard from './DeviceCard';
import { fetchHosts, isPreviewMode } from '../services/api';
import type { Host } from '../services/types';
import { startHostPolling, type HostPollingState } from './hostPolling';

const DeviceList = () => {
  const [pollingState, setPollingState] = useState<HostPollingState<Host>>({ hosts: [], loading: true, error: null });
  useEffect(() => startHostPolling(fetchHosts, setPollingState, isPreviewMode ? 1_000 : undefined), []);
  const { hosts, loading, error } = pollingState;

  return <section className="content-wrap" aria-labelledby="devices-title">
    <div className="page-heading"><div><p className="eyebrow">YOUR NETWORK</p><h1 id="devices-title">Devices</h1><p>Check availability and send power commands to your devices.</p></div>
      {!loading && !error && <span className="count-label">{hosts.length} {hosts.length === 1 ? 'device' : 'devices'}</span>}</div>
    {loading ? <div className="state-panel" role="status">Loading devices…</div>
      : error ? <div className="state-panel error-panel" role="alert"><CircleAlert size={20} aria-hidden="true" /><div><strong>Couldn’t load devices</strong><p>{error}</p></div></div>
        : hosts.length === 0 ? <div className="state-panel empty-panel"><MonitorOff size={26} aria-hidden="true" /><strong>No devices available</strong><p>Ask your administrator to configure a device or grant you access.</p></div>
          : <div className="device-list">{hosts.map(host => <DeviceCard key={host.ID} host={host} />)}</div>}
  </section>;
};

export default DeviceList;
