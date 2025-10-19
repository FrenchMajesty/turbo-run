export type EventType =
  | 'node_created'
  | 'node_ready'
  | 'node_prioritized'
  | 'node_dispatched'
  | 'node_running'
  | 'node_retrying'
  | 'node_completed'
  | 'node_failed'
  | 'priority_queue_add'
  | 'priority_queue_remove';

export interface EventData {
  dependencies?: string[];
  provider?: string;
  estimated_tokens?: number;
  duration?: string;
  error?: string;
  attempt?: number;
  max_retries?: number;
  [key: string]: any;
}

export interface TurboEvent {
  type: EventType;
  node_id: string;
  timestamp: string;
  data?: EventData;
}
