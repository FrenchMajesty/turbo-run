import React from 'react';
import { NodeData } from '../../types/NodeData';
import { WorkNodeCard } from '../WorkNode/WorkNodeCard';
import './style.css';

interface WorkerPoolGridProps {
  workerStates: { [workerId: string]: string };
  nodes: Map<string, NodeData>;
  totalWorkers: number;
}

export const WorkerPoolGrid: React.FC<WorkerPoolGridProps> = ({
  workerStates,
  nodes,
  totalWorkers,
}) => {
  // Create an array of worker IDs from 0 to totalWorkers - 1
  const workerIds = Array.from({ length: totalWorkers }, (_, i) => i);

  return (
    <div className="workerPoolGrid">
      <h2>Worker Pool ({totalWorkers} workers)</h2>
      <div className="gridContainer">
        {workerIds.map((workerId) => {
          const nodeId = workerStates?.[workerId.toString()];
          const node = nodeId ? nodes.get(nodeId) : null;

          return (
            <div key={workerId} className="workerCell">
              {node ? (
                <WorkNodeCard node={node} />
              ) : (
                <div className="emptyWorker">
                  <span className="workerIdLabel">#{workerId}</span>
                  <span className="idleLabel">Idle</span>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
