import React from 'react';
import { NodeData } from '../../types/NodeData';
import './style.css';

interface WorkNodeCardProps {
  node: NodeData;
}

export const WorkNodeCard: React.FC<WorkNodeCardProps> = ({ node }) => {
  const getVariant = () => {
    if (node.status === 'node_completed') return 'completed';
    if (node.status === 'node_failed') return 'failed';
    if (node.status === 'node_running') return 'in_progress';
    return '';
  };

  const isInProgress = node.status === 'node_running';

  return (
    <div className={`workNodeCard ${getVariant()}`}>
      <div className="nodeId">{node.id.substring(0, 8)}</div>

      <div className="tokenRow">
        <span className="tokenCount">{node.data.estimated_tokens || 0}</span>
        <span className="tokenTrail"> tks</span>
      </div>

      <div className="bottomRow">
        {isInProgress ? (
          <span className="status">In progress</span>
        ) : (
          <>
            <span className="duration">{node.data.duration || '0s'}</span>
            {node.data.attempt && node.data.max_retries && (
              <span className="retryCount">
                {node.data.attempt}/{node.data.max_retries} tries
              </span>
            )}
          </>
        )}
      </div>
    </div>
  );
};
