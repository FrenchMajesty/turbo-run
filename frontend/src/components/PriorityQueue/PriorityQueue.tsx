import React from 'react';
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
        <div className={`priorityQueue ${className}`}>
            <h2>Priority Queue</h2>
            {nodeIds.length === 0 ? (
                <div className="emptyMessage">Queue is empty</div>
            ) : (
                <div className="queueContainer">
                    {nodeIds.map((nodeId, index) => {
                        const node = nodes.get(nodeId);
                        return node ? (
                            <WorkNodeCard key={`${nodeId}-${index}`} node={node} />
                        ) : null;
                    })}
                </div>
            )}
        </div>
    );
};
