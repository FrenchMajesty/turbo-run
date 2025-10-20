import React from 'react';
import { useWebSocketStore } from '../stores/useWebSocketStore';
import { useUIStore } from '../stores/useUIStore';
import styles from '../styles/Header.module.css';
import { Badge } from '@/components/ui/badge';

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
    <header className="flex flex-col gap-2 py-4 px-0 border-b border-gray-200">
      <h1 className="text-xl font-bold">TurboRun Visualization</h1>
      <div className="flex flex-row gap-2">
        <Badge variant="default">
          {isConnected ? 'Connected' : 'Disconnected'}
        </Badge>
      </div>
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
