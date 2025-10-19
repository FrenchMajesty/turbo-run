import React from 'react';
import { StatCard } from './StatCard';
import { useStatsStore } from '../stores/useStatsStore';
import styles from '../styles/Stats.module.css';

export const StatsDashboard: React.FC = () => {
  const stats = useStatsStore((state) => state.stats);

  return (
    <div className={styles.statsDashboard}>
      <StatCard title="Graph Size" value={stats.GraphSize} />
      <StatCard title="Queue Size" value={stats.PriorityQueueSize} />
      <StatCard
        title="Workers Busy"
        value={`${stats.WorkersPoolBusy} / ${stats.WorkersPoolSize}`}
      />
      <StatCard title="Launched" value={stats.LaunchedCount} />
      <StatCard title="Completed" value={stats.CompletedCount} />
      <StatCard title="Failed" value={stats.FailedCount} />
    </div>
  );
};
