/**
 * Playback control hook for scenario choreography
 */

import { useCallback } from "react";
import { useChoreographyStore, PlaybackState } from "../stores/choreographyStore";
import { PlaybackSpeed } from "../constants/timing";
import { getScenarioLoader } from "../engine/scenarioLoader";
import { getChoreographer } from "../engine/choreographer";
import { getEventEmitter, ChoreographyEventType } from "../engine/events";

export function usePlayback() {
  const choreography = useChoreographyStore();
  const loader = getScenarioLoader();
  const choreographer = getChoreographer();
  const emitter = getEventEmitter();

  /**
   * Load a scenario from a file
   */
  const loadScenario = useCallback(async (file: File): Promise<void> => {
    try {
      const content = await file.text();
      await loader.loadScenario(content, file.name);
      // State is updated by loader
    } catch (error) {
      console.error("Failed to load scenario:", error);
      throw error;
    }
  }, [loader]);

  /**
   * Start playback
   */
  const play = useCallback(async (): Promise<void> => {
    const state = choreography.state;

    if (state === PlaybackState.LOADED || state === PlaybackState.PAUSED) {
      choreography.setState(PlaybackState.RUNNING);

      if (state === PlaybackState.PAUSED) {
        // Resume
        choreographer.resume();
        await emitter.emit(ChoreographyEventType.PLAYBACK_RESUMED, {});
      } else {
        // Start fresh - emit ready events to kick off choreography
        await emitter.emit(ChoreographyEventType.PLAYBACK_STARTED, {});
        loader.startPlayback();
      }
    }
  }, [choreography, choreographer, emitter, loader]);

  /**
   * Pause playback
   */
  const pause = useCallback(async (): Promise<void> => {
    if (choreography.state === PlaybackState.RUNNING) {
      choreography.setState(PlaybackState.PAUSED);
      choreographer.pause();
      await emitter.emit(ChoreographyEventType.PLAYBACK_PAUSED, {});
    }
  }, [choreography, choreographer, emitter]);

  /**
   * Reset to initial state
   */
  const reset = useCallback(async (): Promise<void> => {
    // Stop all animations
    choreographer.pause();

    // Wait for pending animations
    await choreographer.waitForAnimations();

    // Reset all stores
    choreography.reset();

    // Emit reset event
    await emitter.emit(ChoreographyEventType.PLAYBACK_RESET, {});
  }, [choreography, choreographer, emitter]);

  /**
   * Set playback speed
   */
  const setSpeed = useCallback((speed: PlaybackSpeed): void => {
    choreography.setSpeed(speed);
  }, [choreography]);

  return {
    // State
    state: choreography.state,
    speed: choreography.speed,
    scenarioName: choreography.scenarioName,
    totalNodes: choreography.totalNodes,
    completedNodes: choreography.completedNodes,
    failedNodes: choreography.failedNodes,

    // Actions
    loadScenario,
    play,
    pause,
    reset,
    setSpeed,

    // Computed
    isIdle: choreography.state === PlaybackState.IDLE,
    isLoaded: choreography.state === PlaybackState.LOADED,
    isRunning: choreography.state === PlaybackState.RUNNING,
    isPaused: choreography.state === PlaybackState.PAUSED,
    isCompleted: choreography.state === PlaybackState.COMPLETED,
    canPlay: choreography.state === PlaybackState.LOADED || choreography.state === PlaybackState.PAUSED,
    canPause: choreography.state === PlaybackState.RUNNING,
    canReset: choreography.state !== PlaybackState.IDLE,
  };
}
