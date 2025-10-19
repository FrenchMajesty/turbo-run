import React from 'react';
import { useWebSocketStore } from '../stores/useWebSocketStore';
import { useUIStore } from '../stores/useUIStore';
import styles from '../styles/Header.module.css';

export const Header: React.FC = () => {
  const { isConnected, prepareGraph, startProcessing } = useWebSocketStore();
  const { isPreparing, isGraphPrepared, isProcessing } = useUIStore();
  const getPrepareButtonText = () => {
    if (isPreparing) return 'Preparing...';
    return 'Prepare Graph';
  };

  const getStartButtonText = () => {
    if (isProcessing) return 'Processing...';
    return 'Start Processing';
  };

  const getPrepareButtonClass = () => {
    let className = styles.startButton;
    if (isPreparing) className += ` ${styles.preparing}`;
    if (isGraphPrepared && !isPreparing && !isProcessing) className += ` ${styles.ready}`;
    return className;
  };

  const getStartButtonClass = () => {
    let className = styles.startButton;
    if (isProcessing) className += ` ${styles.processing}`;
    if (isGraphPrepared && !isProcessing) className += ` ${styles.ready}`;
    return className;
  };

  return (
    <header className={styles.header}>
      <h1 className={styles.title}>🚀 TurboRun Real-Time Visualization</h1>
      <span className={`${styles.connectionStatus} ${isConnected ? styles.connected : styles.disconnected}`}>
        {isConnected ? 'Connected' : 'Disconnected'}
      </span>
      <button
        className={getPrepareButtonClass()}
        onClick={prepareGraph}
        disabled={!isConnected || isPreparing || isProcessing}
      >
        {getPrepareButtonText()}
      </button>
      <button
        className={getStartButtonClass()}
        onClick={startProcessing}
        disabled={!isConnected || !isGraphPrepared || isProcessing}
      >
        {getStartButtonText()}
      </button>
    </header>
  );
};
