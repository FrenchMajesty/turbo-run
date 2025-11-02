import React, { useEffect, useMemo, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { NodeData } from '../../types/NodeData';
import { WorkNodeCard } from '../WorkNode/WorkNodeCard';
import './style.css';
import { eventBus } from '@/utils/eventBus';
import { ChoreographyEventType, getEventEmitter } from '@/engine/events';
import { priorityQueue } from '@/utils/heap';

type PriorityQueueProps = {
    className?: string;
}

export const PriorityQueue: React.FC<PriorityQueueProps> = ({ className = '' }) => {
    const [nodes, setNodes] = useState<NodeData[]>([]);
    const emitter = getEventEmitter();
    useEffect(() => {
        // Listen to nodes ready from the graph
        const unsubscribe = emitter.on(ChoreographyEventType.NODE_READY, (data: { node: NodeData }) => {
            priorityQueue.push(data.node, data.node.tokens ?? 0);
        });
        return () => {
            unsubscribe();
        };
    }, []);
    useEffect(() => {
        const pushUnsubscribe = eventBus.subscribe('heap.push', (data: NodeData[]) => {
            setNodes(data);
        });

        const popUnsubscribe = eventBus.subscribe('heap.pop', (data: NodeData[]) => {
            setNodes(data);
        });

        const swapUnsubscribe = eventBus.subscribe('heap.swap', (data: { index1: number, index2: number, data: NodeData[] }) => {
            setNodes(data.data);
        });

        return () => {
            pushUnsubscribe();
            popUnsubscribe();
            swapUnsubscribe();
        };
    }, []);

    return (
        <div className={`flex flex-col gap-2 ${className}`}>
            <h2 className="font-medium">Priority Queue</h2>
            <div className="bg-white rounded-lg p-4 border border-gray-300 overflow-x-auto">
                {nodes.length === 0 ? (
                    <div className="emptyMessage">Queue is empty</div>
                ) : (
                    <div className="flex flex-row gap-4 no-wrap min-w-min">
                        <AnimatePresence mode="popLayout">
                            {nodes.map((node) => (
                                <motion.div
                                    key={node.id}
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
                            ))}
                        </AnimatePresence>
                    </div>
                )}
            </div>
        </div>
    );
};
