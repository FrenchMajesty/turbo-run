import React from 'react';
import { NodeData } from '../../types/NodeData';
import { WorkNodeCard } from '../WorkNode/WorkNodeCard';
import './style.css';

interface WorkerPoolGridProps {
  className?: string;
  workerStates: { [workerId: string]: string };
  nodes: Map<string, NodeData>;
  totalWorkers: number;
}

export const WorkerPoolGrid: React.FC<WorkerPoolGridProps> = ({
  className = '',
  workerStates,
  nodes,
  totalWorkers,
}) => {
  // Create an array of worker IDs from 0 to totalWorkers - 1
  const workerIds = Array.from({ length: totalWorkers }, (_, i) => i);

  return (
    <div className={`flex flex-col gap-2 ${className}`}>
      <h2 className="font-medium">Worker Pool ({totalWorkers} workers)</h2>
      <div className="bg-white rounded-lg border border-gray-300 p-5 flex flex-col max-h-[600px]">
        <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-2.5 overflow-y-auto flex-1 p-1">
          {workerIds.map((workerId) => {
            const nodeId = workerStates?.[workerId.toString()];
            const node = nodeId ? nodes.get(nodeId) : null;

            return (
              <div key={workerId} className="min-h-[100px] flex items-center justify-center">
                {node ? (
                  <WorkNodeCard className="!w-full !h-full" node={node} />
                ) : (
                  <div className="w-full h-full min-h-[100px] bg-gray-100 border-2 border-dashed border-gray-300 rounded-lg flex flex-col items-center justify-center gap-2 transition-all duration-200 hover:bg-gray-200 hover:border-gray-400">
                    <span className="text-sm text-gray-500 font-semibold">#{workerId}</span>
                    <span className="text-xs text-gray-400 uppercase tracking-wide">Idle</span>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
};
