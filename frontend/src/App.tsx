import React, { useState, useEffect } from 'react';
import { useWebSocket } from './hooks/useWebSocket';
import { Header } from './components/Header';
import { StatsDashboard } from './components/StatsDashboard';
import { NodesContainer } from './components/NodesContainer';
import { EventLog } from './components/EventLog';
import { TurboEvent } from './types/TurboEvent';
import { Stats } from './types/Stats';
import { NodeData } from './types/NodeData';
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
  const [isProcessing, setIsProcessing] = useState(false);

  const {
    isConnected,
    startProcessing,
    onTurboEvent,
    onStatsUpdate,
    onInitialStats,
    onProcessingStarted,
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

    onProcessingStarted(() => {
      setIsProcessing(true);
    });
  }, [onTurboEvent, onStatsUpdate, onInitialStats, onProcessingStarted]);

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
      }

      const node = newNodes.get(nodeId)!;
      node.status = type;
      node.data = { ...node.data, ...data };

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

  const handleStartClick = () => {
    startProcessing();
  };

  return (
    <div className="App">
      <div className="container">
        <Header
          isConnected={isConnected}
          isProcessing={isProcessing}
          onStartClick={handleStartClick}
        />

        <StatsDashboard stats={stats} />

        <div className="main-content">
          <NodesContainer nodes={nodes} />
          <EventLog events={events} />
        </div>
      </div>
    </div>
  );
}

export default App;
