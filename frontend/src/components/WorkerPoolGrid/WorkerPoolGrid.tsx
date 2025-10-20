import React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
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
                <AnimatePresence mode="wait">
                  {node ? (
                    <motion.div
                      key={nodeId}
                      initial={{ opacity: 0, scale: 0.8 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={{ opacity: 0, scale: 0.8 }}
                      transition={{
                        duration: 0.3,
                        ease: 'easeInOut'
                      }}
                      className="w-full h-full"
                    >
                      <WorkNodeCard className="!w-full !h-full" node={node} />
                    </motion.div>
                  ) : (
                    <motion.div
                      key={`idle-${workerId}`}
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      transition={{ duration: 0.2 }}
                      className="w-full h-full min-h-[100px] bg-gray-100 border-2 border-dashed border-gray-300 rounded-lg flex flex-col items-center justify-center gap-2 transition-all duration-200 hover:bg-gray-200 hover:border-gray-400"
                    >
                      <span className="text-sm text-gray-500 font-semibold">#{workerId}</span>
                      <span className="text-xs text-gray-400 uppercase tracking-wide">Idle</span>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
};
