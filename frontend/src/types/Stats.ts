export interface Stats {
  GraphSize: number;
  PriorityQueueSize: number;
  PriorityQueueSnapshot?: string[];
  WorkersPoolSize: number;
  WorkersPoolBusy: number;
  WorkerStates?: { [workerId: string]: string }; // Maps worker ID to node ID (empty string if idle)
  LaunchedCount: number;
  CompletedCount: number;
  FailedCount: number;
}
