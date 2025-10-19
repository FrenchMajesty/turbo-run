import React, { useState, useEffect } from 'react';
import { useWebSocket } from './hooks/useWebSocket';
import { Header } from './components/Header';
import { StatsDashboard } from './components/StatsDashboard';
import { NodesContainer } from './components/NodesContainer';
import { GraphCanvas } from './components/GraphCanvas/GraphCanvas';
import { EventLog } from './components/EventLog';
import { TurboEvent } from './types/TurboEvent';
import { Stats } from './types/Stats';
import { NodeData } from './types/NodeData';
import '@xyflow/react/dist/style.css';
import './App.css';
import './tailwind.css';
import { PriorityQueue } from './components/PriorityQueue/PriorityQueue';
import { WorkerPoolGrid } from './components/WorkerPoolGrid/WorkerPoolGrid';

const INITIAL_STATS: Stats = {
  GraphSize: 0,
  PriorityQueueSize: 0,
  WorkersPoolSize: 0,
  WorkersPoolBusy: 0,
  LaunchedCount: 0,
  CompletedCount: 0,
  FailedCount: 0,
};

function App() {
  const [nodes, setNodes] = useState<Map<string, NodeData>>(new Map());
  const [events, setEvents] = useState<TurboEvent[]>([]);
  const [stats, setStats] = useState<Stats>(INITIAL_STATS);
  const [priorityQueueNodes, setPriorityQueueNodes] = useState<string[]>([]);
  const [workerStates, setWorkerStates] = useState<{ [workerId: string]: string }>({});
  const [isPreparing, setIsPreparing] = useState(false);
  const [isGraphPrepared, setIsGraphPrepared] = useState(false);
  const [isProcessing, setIsProcessing] = useState(false);

  const {
    isConnected,
    prepareGraph,
    startProcessing,
    onTurboEvent,
    onStatsUpdate,
    onInitialStats,
    onPriorityQueueUpdate,
    onGraphPreparing,
    onGraphPrepared,
    onGraphCancelled,
    onProcessingStarted,
    onProcessingCompleted,
  } = useWebSocket();

  useEffect(() => {
    onTurboEvent((event: TurboEvent) => {
      handleEvent(event);
      addEventToLog(event);
    });

    onStatsUpdate((newStats: Stats) => {
      setStats(newStats);
    });

    onInitialStats((newStats: Stats) => {
      console.log('Initial stats:', newStats);
      setStats(newStats);
    });

    onPriorityQueueUpdate((nodeIds: string[]) => {
      setPriorityQueueNodes(nodeIds);
    });

    onGraphPreparing(() => {
      setIsPreparing(true);
      setIsGraphPrepared(false);
    });

    onGraphPrepared(() => {
      setIsPreparing(false);
      setIsGraphPrepared(true);
    });

    onGraphCancelled(() => {
      setIsPreparing(false);
      setIsGraphPrepared(false);
      setIsProcessing(false);
      // Clear nodes and events when graph is cancelled
      setNodes(new Map());
      setEvents([]);
      setPriorityQueueNodes([]);
      setWorkerStates({});
    });

    onProcessingStarted(() => {
      setIsProcessing(true);
    });

    onProcessingCompleted(() => {
      setIsProcessing(false);
      setIsGraphPrepared(false);
    });
  }, [onTurboEvent, onStatsUpdate, onInitialStats, onPriorityQueueUpdate, onGraphPreparing, onGraphPrepared, onGraphCancelled, onProcessingStarted, onProcessingCompleted]);

  const handleEvent = (event: TurboEvent) => {
    const nodeId = event.node_id;
    const type = event.type;
    const data = event.data || {};

    // Handle priority queue events
    if (type === 'priority_queue_add') {
      setPriorityQueueNodes((prev) => [...prev, nodeId]);
      return;
    }

    if (type === 'priority_queue_remove') {
      setPriorityQueueNodes((prev) => prev.filter((id) => id !== nodeId));
      return;
    }

    // Handle worker state changes - track which worker is working on which node
    if (type === 'node_running' && data.worker_id !== undefined) {
      const workerId = data.worker_id;
      setWorkerStates((prev) => ({
        ...prev,
        [workerId.toString()]: nodeId,
      }));
    }

    // Clear worker state when node completes or fails
    if (type === 'node_completed' || type === 'node_failed') {
      // Find and clear the worker that was working on this node
      setWorkerStates((prev) => {
        const newStates = { ...prev };
        for (const [workerId, workerNodeId] of Object.entries(newStates)) {
          if (workerNodeId === nodeId) {
            delete newStates[workerId];
            break;
          }
        }
        return newStates;
      });
    }

    setNodes((prevNodes) => {
      const newNodes = new Map(prevNodes);

      if (!newNodes.has(nodeId)) {
        newNodes.set(nodeId, {
          id: nodeId,
          status: type,
          dependencies: data.dependencies || [],
          provider: data.provider || 'unknown',
          tokens: data.estimated_tokens || 0,
          data: {},
        });
      } else {
        // Create a NEW node object (don't mutate the existing one!)
        const existingNode = newNodes.get(nodeId)!;
        newNodes.set(nodeId, {
          ...existingNode,
          status: type,
          data: { ...existingNode.data, ...data },
        });
      }

      return newNodes;
    });

    // Auto-remove completed nodes after 3 seconds
    if (type === 'node_completed') {
      setTimeout(() => {
        setNodes((prevNodes) => {
          const newNodes = new Map(prevNodes);
          newNodes.delete(nodeId);
          return newNodes;
        });
      }, 3000);
    }
  };

  const addEventToLog = (event: TurboEvent) => {
    setEvents((prevEvents) => {
      const newEvents = [event, ...prevEvents];
      // Keep only last 50 events
      return newEvents.slice(0, 50);
    });
  };

  const handlePrepareClick = () => {
    prepareGraph();
  };

  const handleStartClick = () => {
    startProcessing();
  };

  return (
    <div className="App">
      <div className="container">
        <Header
          isConnected={isConnected}
          isPreparing={isPreparing}
          isGraphPrepared={isGraphPrepared}
          isProcessing={isProcessing}
          onPrepareClick={handlePrepareClick}
          onStartClick={handleStartClick}
        />

        <StatsDashboard stats={stats} />

        <div className="flex flex-col gap-4">
          <div className="flex flex-row gap-4">
            <WorkerPoolGrid
              workerStates={workerStates}
              nodes={nodes}
              totalWorkers={stats.WorkersPoolSize}
            />
            <PriorityQueue nodeIds={priorityQueueNodes} nodes={nodes} />
            <GraphCanvas nodes={nodes} />

          </div>
          <div className="flex flex-row gap-4">
            <EventLog events={events} />
          </div>
        </div>
      </div>
    </div>
  );
}

export default App;
