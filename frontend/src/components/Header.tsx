import React from 'react';
import styles from '../styles/Header.module.css';

interface HeaderProps {
  isConnected: boolean;
  isProcessing: boolean;
  onStartClick: () => void;
}

export const Header: React.FC<HeaderProps> = ({ isConnected, isProcessing, onStartClick }) => {
  const getButtonText = () => {
    if (isProcessing) return 'Processing...';
    if (!isConnected) return 'Start Processing';
    return 'Start Processing';
  };

  const getButtonClass = () => {
    let className = styles.startButton;
    if (isProcessing) className += ` ${styles.processing}`;
    return className;
  };

  return (
    <header className={styles.header}>
      <h1 className={styles.title}>🚀 TurboRun Real-Time Visualization</h1>
      <span className={`${styles.connectionStatus} ${isConnected ? styles.connected : styles.disconnected}`}>
        {isConnected ? 'Connected' : 'Disconnected'}
      </span>
      <button
        className={getButtonClass()}
        onClick={onStartClick}
        disabled={!isConnected || isProcessing}
      >
        {getButtonText()}
      </button>
    </header>
  );
};
