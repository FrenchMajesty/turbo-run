import React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { NodeData } from '../../types/NodeData';
import { WorkNodeCard } from '../WorkNode/WorkNodeCard';
import './style.css';

type PriorityQueueProps = {
    className?: string;
    nodeIds: string[];
    nodes: Map<string, NodeData>;
}

export const PriorityQueue: React.FC<PriorityQueueProps> = ({ className = '', nodeIds, nodes }) => {
    return (
        <div className={`flex flex-col gap-2 ${className}`}>
            <h2 className="font-medium">Priority Queue</h2>
            <div className="bg-white rounded-lg p-4 border border-gray-300 overflow-x-auto">
                {nodeIds.length === 0 ? (
                    <div className="emptyMessage">Queue is empty</div>
                ) : (
                    <div className="flex flex-row gap-4 no-wrap min-w-min">
                        <AnimatePresence mode="popLayout">
                            {nodeIds.map((nodeId, index) => {
                                const node = nodes.get(nodeId);
                                return node ? (
                                    <motion.div
                                        key={nodeId}
                                        initial={{ x: -100, opacity: 0 }}
                                        animate={{ x: 0, opacity: 1 }}
                                        exit={{ x: 100, opacity: 0 }}
                                        transition={{
                                            duration: 0.3,
                                            ease: 'easeInOut'
                                        }}
                                        layout
                                    >
                                        <WorkNodeCard node={node} />
                                    </motion.div>
                                ) : null;
                            })}
                        </AnimatePresence>
                    </div>
                )}
            </div>
        </div>
    );
};
