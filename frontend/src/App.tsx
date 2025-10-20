import React, { useEffect } from 'react';
import { useWebSocketStore } from './stores/useWebSocketStore';
import { useNodesStore } from './stores/useNodesStore';
import { useWorkersStore } from './stores/useWorkersStore';
import { useQueueStore } from './stores/useQueueStore';
import { useStatsStore } from './stores/useStatsStore';
import { useWebSocketSync } from './hooks/useWebSocketSync';
import { Header } from './components/Header';
import { StatsDashboard } from './components/StatsDashboard';
import { GraphCanvas } from './components/GraphCanvas/GraphCanvas';
import { EventLog } from './components/EventLog';
import { PriorityQueue } from './components/PriorityQueue/PriorityQueue';
import { WorkerPoolGrid } from './components/WorkerPoolGrid/WorkerPoolGrid';
import '@xyflow/react/dist/style.css';
import './App.css';
import './tailwind.css';

function App() {
  // Initialize WebSocket connection
  const { initialize, disconnect } = useWebSocketStore();

  // Subscribe to stores (only for components that still need props)
  const nodes = useNodesStore((state) => state.nodes);
  const workerStates = useWorkersStore((state) => state.workerStates);
  const priorityQueueNodes = useQueueStore((state) => state.priorityQueueNodes);
  const stats = useStatsStore((state) => state.stats);

  // Sync WebSocket events to stores
  useWebSocketSync();

  // Initialize WebSocket on mount
  useEffect(() => {
    initialize();
    return () => disconnect();
  }, [initialize, disconnect]);

  return (
    <div className="App">
      <div className="container flex flex-col gap-8">
        <Header
          launchedCount={stats.LaunchedCount}
          completedCount={stats.CompletedCount}
          failedCount={stats.FailedCount}
        />

        <div className="flex flex-col gap-4">
          <div className="grid grid-cols-[25vw_25vw_auto] gap-4">
            <WorkerPoolGrid
              className='w-[25vw]'
              workerStates={workerStates}
              nodes={nodes}
              totalWorkers={stats.WorkersPoolSize}
            />
            <PriorityQueue
              className='w-[25vw]'
              nodeIds={priorityQueueNodes}
              nodes={nodes}
            />
            <GraphCanvas nodes={nodes} className='flex-1' />
          </div>
          <div className="flex flex-row gap-4">
            <EventLog />
          </div>
        </div>
      </div>
    </div>
  );
}

export default App;
