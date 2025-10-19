import React from 'react';
import { WorkNodeCard } from './WorkNode/WorkNodeCard';
import { NodeData } from '../types/NodeData';
import styles from '../styles/Node.module.css';

interface NodesContainerProps {
  nodes: Map<string, NodeData>;
}

export const NodesContainer: React.FC<NodesContainerProps> = ({ nodes }) => {
  const nodesList = Array.from(nodes.values()).reverse();

  return (
    <div className={styles.nodesContainer}>
      <h2>📦 Work Nodes</h2>
      <div className={styles.nodesWrapper}>
        {nodesList.map((node) => (
          <WorkNodeCard key={node.id} node={node} />
        ))}
      </div>
    </div>
  );
};
