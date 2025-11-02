/**
 * Centralized timing constants for all animations
 */

/** Base duration for all animations in milliseconds */
export const ANIMATION_DURATION_MS = 300;

/** Default playback speed multiplier */
export const DEFAULT_PLAYBACK_SPEED = 1.0;

/** Available playback speed options */
export const PLAYBACK_SPEEDS = [0.25, 0.5, 1, 2, 4] as const;
export type PlaybackSpeed = (typeof PLAYBACK_SPEEDS)[number];
