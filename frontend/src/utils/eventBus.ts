type EventCallback<T = any> = (data: T) => Promise<void> | void;

class EventBus {
  private events: Map<string, Set<EventCallback>> = new Map();

  /**
   * Subscribes to an event.
   * @param event - The event to subscribe to.
   * @param callback - The callback to execute when the event is published.
   * @returns A function to unsubscribe from the event.
   */
  subscribe<T>(event: string, callback: EventCallback<T>): () => void {
    if (!this.events.has(event)) {
      this.events.set(event, new Set());
    }
    this.events.get(event)!.add(callback);

    // Return unsubscribe function
    return () => {
      this.events.get(event)?.delete(callback);
    };
  }

  /**
   * Publishes an event.
   * @param event - The event to publish.
   * @param data - The data to publish.
   */
  publishAsync<T>(event: string, data?: T): void {
    this.events.get(event)?.forEach(cb => cb(data));
  }

  /**
   * Publishes an event synchronously and waits for all callbacks to complete.
   * @param event - The event to publish.
   * @param data - The data to publish.
   */
  async publish<T>(event: string, data?: T): Promise<void> {
    const callbacks = Array.from(this.events.get(event) || []);
    await Promise.all(callbacks.map(cb => cb(data)));
  }

  /**
   * Clears an event. If no event is provided, clears all events.
   * @param event - The event to clear.
   */
  clear(event?: string): void {
    if (event) {
      this.events.delete(event);
    } else {
      this.events.clear();
    }
  }
}

export const eventBus = new EventBus();
