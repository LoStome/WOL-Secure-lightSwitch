import React, { useEffect, useState } from 'react';
import DeviceCard from './DeviceCard';
import { fetchHosts } from '../services/api';
import type { Host } from '../services/api';
import { PowerOff } from 'lucide-react'; // use an icon for empty state

const DeviceList: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let intervalId: ReturnType<typeof setInterval>;

    const loadHosts = async () => {
      try {
        const data = await fetchHosts();
        setHosts(data || []);
      } catch (err: any) {
        setError(err.message || 'Error fetching hosts');
      } finally {
        setLoading(false);
      }
    };

    loadHosts();
    
    // Poll every 10 seconds
    intervalId = setInterval(loadHosts, 10000);

    return () => {
      if (intervalId) clearInterval(intervalId);
    };
  }, []);

  if (loading) {
    return (
      <div className="flex justify-center items-center py-20">
        <div className="w-10 h-10 border-4 border-transparent border-t-zinc-400 border-r-zinc-400 rounded-full animate-spin"></div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-500/10 text-red-500 border border-red-500/50 p-4 rounded-xl text-center max-w-2xl mx-auto">
        <p className="font-semibold">Error</p>
        <p className="text-sm">{error}</p>
      </div>
    );
  }

  if (!hosts || hosts.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-zinc-500 gap-4">
        <PowerOff className="w-16 h-16 opacity-50" />
        <p className="text-lg">No devices configured.</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6 max-w-7xl mx-auto">
      {hosts.map((host) => (
        <DeviceCard key={host.ID} host={host} />
      ))}
    </div>
  );
};

export default DeviceList;
