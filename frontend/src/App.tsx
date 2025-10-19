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
    });

    onProcessingStarted(() => {
      setIsProcessing(true);
    });

    onProcessingCompleted(() => {
      setIsProcessing(false);
      setIsGraphPrepared(false);
    });
  }, [onTurboEvent, onStatsUpdate, onInitialStats, onGraphPreparing, onGraphPrepared, onGraphCancelled, onProcessingStarted, onProcessingCompleted]);

  const handleEvent = (event: TurboEvent) => {
    const nodeId = event.node_id;
    const type = event.type;
    const data = event.data || {};

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

        <div className="main-content">
          <EventLog events={events} />
          <GraphCanvas nodes={nodes} />
        </div>
      </div>
    </div>
  );
}

export default App;
