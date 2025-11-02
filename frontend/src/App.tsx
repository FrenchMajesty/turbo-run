import React, { useEffect } from 'react';
import { useNodesStore } from './stores/useNodesStore';
import { useWorkersStore } from './stores/useWorkersStore';
import { useQueueStore } from './stores/useQueueStore';
import { getChoreographer } from './engine/choreographer';
import { Header } from './components/Header';
import { GraphCanvas } from './components/GraphCanvas/GraphCanvas';
import { EventLog } from './components/EventLog';
import { PriorityQueue } from './components/PriorityQueue/PriorityQueue';
import { WorkerPoolGrid } from './components/WorkerPoolGrid/WorkerPoolGrid';
import '@xyflow/react/dist/style.css';
import './App.css';
import './tailwind.css';

function App() {
  // Subscribe to stores (only for components that still need props)
  const nodes = useNodesStore((state) => state.nodes);
  const workerStates = useWorkersStore((state) => state.workerStates);
  const priorityQueueNodes = useQueueStore((state) => state.priorityQueueNodes);

  // Initialize choreographer on mount
  useEffect(() => {
    const choreographer = getChoreographer();
    return () => {
      choreographer.cleanup();
    };
  }, []);

  return (
    <div className="App">
      <div className="container flex flex-col gap-8">
        <Header />

        <div className="flex flex-col gap-4">
          <div className="grid grid-cols-[25vw_25vw_auto] gap-4">
            <WorkerPoolGrid
              className='w-[25vw]'
              workerStates={workerStates}
              nodes={nodes}
              totalWorkers={120}
            />
            <PriorityQueue className='w-[25vw]' />
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
