import { memo } from 'react';
import { NodeData } from '../../types/NodeData';
import { WorkNodeCard } from '../WorkNode/WorkNodeCard';
import { Handle, Position } from '@xyflow/react';

// Custom node component with fixed dimensions to prevent resize loops
// Use memo with custom comparison to ensure we re-render when data.status changes
export const WorkNodeComponent = memo(
    ({ data }: { data: NodeData }) => {
        return (
            <div style={{ width: '97px', height: '75px', position: 'relative' }}>
                <Handle
                    type="target"
                    position={Position.Left}
                    style={{ opacity: 0, pointerEvents: 'none' }}
                />
                <WorkNodeCard node={data} />
                <Handle
                    type="source"
                    position={Position.Right}
                    style={{ opacity: 0, pointerEvents: 'none' }}
                />
            </div>
        );
    },
    (prevProps, nextProps) => {
        // Re-render if status or any data property changes
        return (
            prevProps.data.id === nextProps.data.id &&
            prevProps.data.status === nextProps.data.status &&
            JSON.stringify(prevProps.data.data) === JSON.stringify(nextProps.data.data)
        );
    }
);


WorkNodeComponent.displayName = 'WorkNodeComponent';
