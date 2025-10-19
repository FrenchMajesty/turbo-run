export interface Stats {
  GraphSize: number;
  PriorityQueueSize: number;
  PriorityQueueSnapshot?: string[];
  WorkersPoolSize: number;
  WorkersPoolBusy: number;
  LaunchedCount: number;
  CompletedCount: number;
  FailedCount: number;
}
