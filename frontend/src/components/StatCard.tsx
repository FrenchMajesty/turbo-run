import React from 'react';
import styles from '../styles/Stats.module.css';

interface StatCardProps {
  title: string;
  value: string | number;
}

export const StatCard: React.FC<StatCardProps> = ({ title, value }) => {
  return (
    <div className={styles.statCard}>
      <h3>{title}</h3>
      <div className={styles.value}>{value}</div>
    </div>
  );
};
