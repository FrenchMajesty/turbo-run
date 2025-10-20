import React, { useMemo } from 'react';
import { useWebSocketStore } from '../stores/useWebSocketStore';
import { useUIStore } from '../stores/useUIStore';
import { Badge } from '@/components/ui/badge';
import { Button } from './ui/button';
import { Stats } from '../types/Stats';

type HeaderProps = {
  launchedCount: number;
  completedCount: number;
  failedCount: number;
};

export const Header: React.FC<HeaderProps> = ({ launchedCount, completedCount, failedCount }) => {
  const { isConnected, prepareGraph, startProcessing } = useWebSocketStore();
  const { isPreparing, isGraphPrepared, isProcessing } = useUIStore();
  const startButtonText = useMemo(() => {
    return isProcessing ? 'Processing...' : 'Start Processing';
  }, []);
  const prepareButtonText = useMemo(() => {
    return isPreparing ? 'Preparing...' : 'Prepare Graph';
  }, []);


  return (
    <header className="flex flex-col gap-2 py-4 px-0 border-b border-gray-200">
      <div className="flex flex-row gap-2">
        <h1 className="text-xl font-bold">TurboRun Visualization</h1>
      </div>
      <div className="flex flex-row justify-between">
        <div className="flex flex-row gap-2">
          <Button
            onClick={prepareGraph}
            variant="outline"
            disabled={!isConnected || isPreparing || isProcessing}
          >
            {prepareButtonText}
          </Button>
          <Button
            onClick={startProcessing}
            disabled={!isConnected || !isGraphPrepared || isProcessing}
          >
            {startButtonText}
          </Button>
        </div>
        <div className="flex flex-row gap-2 h-fit">
          <Badge variant={isConnected ? 'success' : 'yellow'}>
            Status: {isConnected ? 'Connected' : 'Disconnected'}
          </Badge>
          <Badge variant="gray">
            Launched: {launchedCount}
          </Badge>
          <Badge variant="gray">
            Completed: {completedCount}
          </Badge>
          <Badge variant="destructive">
            Failed: {failedCount}
          </Badge>
        </div>
      </div>
    </header>
  );
};
