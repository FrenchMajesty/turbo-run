import React from 'react';
import styles from '../styles/Header.module.css';

interface HeaderProps {
  isConnected: boolean;
  isPreparing: boolean;
  isGraphPrepared: boolean;
  isProcessing: boolean;
  onPrepareClick: () => void;
  onStartClick: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  isConnected,
  isPreparing,
  isGraphPrepared,
  isProcessing,
  onPrepareClick,
  onStartClick
}) => {
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
        onClick={onPrepareClick}
        disabled={!isConnected || isPreparing || isProcessing}
      >
        {getPrepareButtonText()}
      </button>
      <button
        className={getStartButtonClass()}
        onClick={onStartClick}
        disabled={!isConnected || !isGraphPrepared || isProcessing}
      >
        {getStartButtonText()}
      </button>
    </header>
  );
};
