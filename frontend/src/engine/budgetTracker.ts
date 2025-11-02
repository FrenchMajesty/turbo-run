/**
 * Budget tracker for simulating rate limits
 * Mimics backend's ConsumptionTracker behavior
 */

import { Provider, RateLimitConfig } from "../types/scenario";
import { getEventEmitter, ChoreographyEventType } from "./events";

interface ProviderBudget {
  config: RateLimitConfig;
  tokensConsumed: number;
  requestsConsumed: number;
  windowStartTime: number;
}

export class BudgetTracker {
  private budgets: Map<Provider, ProviderBudget> = new Map();
  private timers: Map<Provider, NodeJS.Timeout> = new Map();
  private emitter = getEventEmitter();
  private isPaused = false;

  /**
   * Initialize budget tracking for a provider
   */
  initialize(provider: Provider, config: RateLimitConfig): void {
    this.budgets.set(provider, {
      config,
      tokensConsumed: 0,
      requestsConsumed: 0,
      windowStartTime: Date.now(),
    });

    // Clear existing timer if any
    const existingTimer = this.timers.get(provider);
    if (existingTimer) {
      clearInterval(existingTimer);
    }

    // Start window reset timer
    const timer = setInterval(() => {
      if (!this.isPaused) {
        this.resetWindow(provider);
      }
    }, config.window_ms);

    this.timers.set(provider, timer);
  }

  /**
   * Check if a node can be launched given current budget constraints
   */
  canLaunch(provider: Provider, estimatedTokens: number): boolean {
    const budget = this.budgets.get(provider);
    if (!budget) {
      console.warn(`No budget configured for provider: ${provider}`);
      return true; // Allow if not configured
    }

    const tokensAvailable = budget.config.tokens_per_window - budget.tokensConsumed;
    const requestsAvailable = budget.config.requests_per_window - budget.requestsConsumed;

    return tokensAvailable >= estimatedTokens && requestsAvailable >= 1;
  }

  /**
   * Record consumption for a launched node
   */
  recordConsumption(provider: Provider, tokens: number): void {
    const budget = this.budgets.get(provider);
    if (!budget) {
      console.warn(`No budget configured for provider: ${provider}`);
      return;
    }

    budget.tokensConsumed += tokens;
    budget.requestsConsumed += 1;
  }

  /**
   * Get current budget stats for a provider
   */
  getStats(provider: Provider): {
    tokensConsumed: number;
    tokensRemaining: number;
    requestsConsumed: number;
    requestsRemaining: number;
    windowProgress: number; // 0-1
  } | null {
    const budget = this.budgets.get(provider);
    if (!budget) {
      return null;
    }

    const elapsed = Date.now() - budget.windowStartTime;
    const windowProgress = Math.min(elapsed / budget.config.window_ms, 1);

    return {
      tokensConsumed: budget.tokensConsumed,
      tokensRemaining: budget.config.tokens_per_window - budget.tokensConsumed,
      requestsConsumed: budget.requestsConsumed,
      requestsRemaining: budget.config.requests_per_window - budget.requestsConsumed,
      windowProgress,
    };
  }

  /**
   * Reset window for a provider
   */
  private resetWindow(provider: Provider): void {
    const budget = this.budgets.get(provider);
    if (!budget) {
      return;
    }

    budget.tokensConsumed = 0;
    budget.requestsConsumed = 0;
    budget.windowStartTime = Date.now();

    // Emit budget reset event
    this.emitter.emit(ChoreographyEventType.BUDGET_RESET, { provider });
  }

  /**
   * Pause budget tracking (stop timers)
   */
  pause(): void {
    this.isPaused = true;
  }

  /**
   * Resume budget tracking
   */
  resume(): void {
    this.isPaused = false;
  }

  /**
   * Stop all timers and clear budgets
   */
  cleanup(): void {
    this.timers.forEach((timer) => {
      clearInterval(timer);
    });
    this.timers.clear();
    this.budgets.clear();
    this.isPaused = false;
  }

  /**
   * Force reset of all budgets (useful for testing)
   */
  forceResetAll(): void {
    this.budgets.forEach((_, provider) => {
      this.resetWindow(provider);
    });
  }
}

// Global singleton
let globalTracker: BudgetTracker | null = null;

/**
 * Get the global budget tracker instance
 */
export function getBudgetTracker(): BudgetTracker {
  if (!globalTracker) {
    globalTracker = new BudgetTracker();
  }
  return globalTracker;
}

/**
 * Reset the global budget tracker
 */
export function resetBudgetTracker(): void {
  globalTracker?.cleanup();
  globalTracker = null;
}
