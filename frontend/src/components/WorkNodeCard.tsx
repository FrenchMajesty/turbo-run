import React from 'react';
import { NodeData } from '../types/NodeData';
import styles from '../styles/Node.module.css';

interface WorkNodeCardProps {
  node: NodeData;
}

export const WorkNodeCard: React.FC<WorkNodeCardProps> = ({ node }) => {
  const getStatusClass = () => {
    const status = node.status.replace('node_', '');
    return `${styles.node} ${styles[status]}`;
  };

  const getStatusLabel = () => {
    return node.status.replace('node_', '');
  };

  return (
    <div className={getStatusClass()}>
      <div>
        <span className={styles.nodeId}>{node.id.substring(0, 8)}</span>
        <span className={styles.nodeStatus}>{getStatusLabel()}</span>
      </div>

      {node.data.estimated_tokens && (
        <div className={styles.nodeDetails}>Tokens: {node.data.estimated_tokens}</div>
      )}

      {node.data.duration && (
        <div className={styles.nodeDetails}>Duration: {node.data.duration}</div>
      )}

      {node.data.error && (
        <div className={`${styles.nodeDetails} ${styles.error}`}>
          Error: {node.data.error}
        </div>
      )}

      {node.data.attempt && (
        <div className={styles.nodeDetails}>
          Retry: {node.data.attempt}/{node.data.max_retries}
        </div>
      )}

      {node.dependencies && node.dependencies.length > 0 && (
        <div className={styles.nodeDetails}>
          Dependencies: {node.dependencies.length}
        </div>
      )}
    </div>
  );
};
