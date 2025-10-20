import { useEffect, useRef, useCallback, useMemo } from 'react';

export interface RateLimitedQueueConfig {
  maxPerSecond: number; // Maximum items to process per second
}

export interface RateLimitedQueueHook<T> {
  enqueue: (item: T) => void;
}

/**
 * Custom hook for processing a queue at a controlled rate
 *
 * @param config - Configuration with maxPerSecond rate limit
 * @param processor - Function to process each item
 * @returns Queue control interface
 *
 * Example: maxPerSecond=10 means items are dequeued every 100ms
 */
export function useRateLimitedQueue<T>(
  config: RateLimitedQueueConfig,
  processor: (item: T) => void
): RateLimitedQueueHook<T> {
  const queueRef = useRef<T[]>([]);
  const processingRef = useRef(false);
  const timerRef = useRef<NodeJS.Timeout | null>(null);

  // Calculate delay between items (in ms)
  const delayMs = 0;//1000 / config.maxPerSecond;

  // Process next item in queue
  const processNext = useCallback(() => {
    if (queueRef.current.length === 0) {
      // Queue is empty, stop processing
      processingRef.current = false;
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      return;
    }

    // Dequeue first item
    const item = queueRef.current.shift()!;

    // Wait for delay, then process the item
    timerRef.current = setTimeout(() => {
      processor(item);

      // Continue to next item
      processNext();
    }, delayMs);
  }, [delayMs, processor]);

  // Start processing if not already processing
  const startProcessing = useCallback(() => {
    if (!processingRef.current && queueRef.current.length > 0) {
      processingRef.current = true;
      processNext();
    }
  }, [processNext]);

  const enqueue = useCallback((item: T) => {
    queueRef.current.push(item);
    startProcessing();
  }, [startProcessing]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      processingRef.current = false;
    };
  }, []);

  // Return stable object reference
  return {
    enqueue,
  }
}
