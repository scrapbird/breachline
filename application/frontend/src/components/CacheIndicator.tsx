import React, { useEffect, useState } from 'react';
// @ts-ignore - Wails generated bindings
import * as AppAPI from '../../wailsjs/go/app/App';
import './CacheIndicator.css';

interface CacheStats {
  totalSize: number;
  maxSize: number;
  usagePercent: number;
  entryCount: number;
}

interface CacheIndicatorProps {
  visible: boolean;
}

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

export const CacheIndicator: React.FC<CacheIndicatorProps> = ({ visible }) => {
  const [stats, setStats] = useState<CacheStats>({
    totalSize: 0,
    maxSize: 0,
    usagePercent: 0,
    entryCount: 0,
  });

  useEffect(() => {
    if (!visible) return;

    const fetchStats = async () => {
      try {
        const result = await AppAPI.GetCacheStats();
        setStats(result);
      } catch (e) {
        console.error('Failed to fetch cache stats:', e);
      }
    };

    // Fetch immediately
    fetchStats();

    // Poll every 2 seconds while visible
    const interval = setInterval(fetchStats, 2000);

    return () => clearInterval(interval);
  }, [visible]);

  if (!visible) return null;

  const usagePercent = Math.min(100, Math.max(0, stats.usagePercent));

  return (
    <div 
      className="cache-indicator" 
      title={`Cache: ${formatBytes(stats.totalSize)} / ${formatBytes(stats.maxSize)} (${usagePercent.toFixed(1)}%)\n${stats.entryCount} entries`}
    >
      <div className="cache-indicator-label">Cache</div>
      <div className="cache-indicator-bar-container">
        <div 
          className="cache-indicator-bar" 
          style={{ width: `${usagePercent}%` }}
        />
      </div>
      <div className="cache-indicator-percent">{Math.round(usagePercent)}%</div>
    </div>
  );
};
