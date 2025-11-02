import React, { useRef } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from './ui/button';
import { usePlayback } from '../hooks/usePlayback';
import { PlaybackState } from '../stores/choreographyStore';
import { PLAYBACK_SPEEDS } from '../constants/timing';

export const Header: React.FC = () => {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const {
    state,
    speed,
    scenarioName,
    totalNodes,
    completedNodes,
    failedNodes,
    loadScenario,
    play,
    pause,
    reset,
    setSpeed,
    canPlay,
    canPause,
    canReset,
  } = usePlayback();

  const handleFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      try {
        await loadScenario(file);
      } catch (error) {
        alert(`Failed to load scenario: ${error}`);
      }
    }
  };

  const getStateText = () => {
    switch (state) {
      case PlaybackState.IDLE:
        return 'No scenario loaded';
      case PlaybackState.LOADED:
        return 'Ready to play';
      case PlaybackState.RUNNING:
        return 'Playing...';
      case PlaybackState.PAUSED:
        return 'Paused';
      case PlaybackState.COMPLETED:
        return 'Completed';
      default:
        return '';
    }
  };

  const getStateBadgeVariant = () => {
    switch (state) {
      case PlaybackState.IDLE:
        return 'gray';
      case PlaybackState.LOADED:
        return 'yellow';
      case PlaybackState.RUNNING:
        return 'success';
      case PlaybackState.PAUSED:
        return 'yellow';
      case PlaybackState.COMPLETED:
        return 'success';
      default:
        return 'gray';
    }
  };

  return (
    <header className="flex flex-col gap-2 py-4 px-0 border-b border-gray-200">
      <div className="flex flex-row gap-2">
        <h1 className="text-xl font-bold">TurboRun Visualization</h1>
        {scenarioName && (
          <span className="text-sm text-gray-500 self-center">
            ({scenarioName})
          </span>
        )}
      </div>
      <div className="flex flex-row justify-between">
        <div className="flex flex-row gap-2">
          <input
            ref={fileInputRef}
            type="file"
            accept=".json"
            onChange={handleFileChange}
            className="hidden"
          />
          <Button
            onClick={() => fileInputRef.current?.click()}
            variant="outline"
            disabled={state === PlaybackState.RUNNING}
          >
            Load Scenario
          </Button>
          <Button
            onClick={play}
            disabled={!canPlay}
          >
            {state === PlaybackState.PAUSED ? 'Resume' : 'Play'}
          </Button>
          <Button
            onClick={pause}
            variant="outline"
            disabled={!canPause}
          >
            Pause
          </Button>
          <Button
            onClick={reset}
            variant="outline"
            disabled={!canReset}
          >
            Reset
          </Button>
          <select
            value={speed}
            onChange={(e) => setSpeed(Number(e.target.value) as typeof PLAYBACK_SPEEDS[number])}
            className="px-2 py-1 border rounded"
            disabled={state === PlaybackState.IDLE}
          >
            {PLAYBACK_SPEEDS.map((s) => (
              <option key={s} value={s}>
                {s}x
              </option>
            ))}
          </select>
        </div>
        <div className="flex flex-row gap-2 h-fit">
          <Badge variant={getStateBadgeVariant()}>
            {getStateText()}
          </Badge>
          {state !== PlaybackState.IDLE && (
            <>
              <Badge variant="gray">
                Total: {totalNodes}
              </Badge>
              <Badge variant="success">
                Completed: {completedNodes}
              </Badge>
              <Badge variant="destructive">
                Failed: {failedNodes}
              </Badge>
            </>
          )}
        </div>
      </div>
    </header>
  );
};
