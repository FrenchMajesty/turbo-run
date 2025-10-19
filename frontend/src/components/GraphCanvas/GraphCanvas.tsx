import React, { useMemo, memo, useEffect, useRef } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  NodeTypes,
  Node,
  Handle,
  Position,
  useNodesState,
  useEdgesState,
  useReactFlow,
  ReactFlowProvider,
} from '@xyflow/react';
import { NodeData } from '../../types/NodeData';
import { WorkNodeCard } from '../WorkNode/WorkNodeCard';
import { getLayoutedElements } from '../../utils/graphLayout';
import styles from './GraphCanvas.module.css';

interface GraphCanvasProps {
  nodes: Map<string, NodeData>;
}

// Custom node component with fixed dimensions to prevent resize loops
const WorkNodeComponent = memo(({ data }: { data: NodeData }) => {
  return (
    <div style={{ width: '97px', height: '75px', position: 'relative' }}>
      <Handle
        type="target"
        position={Position.Top}
        style={{ opacity: 0, pointerEvents: 'none' }}
      />
      <WorkNodeCard node={data} />
      <Handle
        type="source"
        position={Position.Bottom}
        style={{ opacity: 0, pointerEvents: 'none' }}
      />
    </div>
  );
});

WorkNodeComponent.displayName = 'WorkNodeComponent';

// Register custom node type (outside component to prevent recreation)
const nodeTypes: NodeTypes = {
  workNode: WorkNodeComponent,
};

const GraphCanvasInner: React.FC<GraphCanvasProps> = ({ nodes }) => {
  const reactFlowInstance = useReactFlow();
  const hasInitialized = useRef(false);

  // Create a stable key based only on node IDs (not status changes)
  const nodeIdsKey = useMemo(() => {
    return Array.from(nodes.keys()).sort().join(',');
  }, [nodes]);

  // Only recalculate layout when nodes are added/removed, NOT on status updates
  const layoutedElements = useMemo(() => {
    return getLayoutedElements(nodes);
  }, [nodeIdsKey]);

  const [nodesState, setNodes, onNodesChange] = useNodesState(layoutedElements.nodes);
  const [edgesState, setEdges, onEdgesChange] = useEdgesState(layoutedElements.edges);

  // Update nodes and edges when layout changes
  useEffect(() => {
    setNodes(layoutedElements.nodes);
    setEdges(layoutedElements.edges);

    // Only fitView on first render
    if (!hasInitialized.current && layoutedElements.nodes.length > 0) {
      setTimeout(() => {
        reactFlowInstance.fitView({ duration: 200, maxZoom: 1 });
      }, 0);
      hasInitialized.current = true;
    }
  }, [layoutedElements, setNodes, setEdges, reactFlowInstance]);

  return (
    <div className={styles.graphContainer}>
      <h2>🕸️ Dependency Graph</h2>
      <div className={styles.reactFlowWrapper}>
        <ReactFlow
          nodes={nodesState}
          edges={edgesState}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          nodeTypes={nodeTypes}
          minZoom={0.2}
          maxZoom={2}
          defaultEdgeOptions={{
            type: 'smoothstep',
          }}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={true}
          proOptions={{ hideAttribution: true }}
        >
          <Background />
          <Controls />
          <MiniMap
            nodeColor={(node: Node<NodeData>) => {
              if (node.data.status === 'node_completed') return '#d4edda';
              if (node.data.status === 'node_failed') return '#f8d7da';
              if (node.data.status === 'node_running') return '#d1ecf1';
              return '#f1f1f1';
            }}
            maskColor="rgba(0, 0, 0, 0.1)"
          />
        </ReactFlow>
      </div>
    </div>
  );
};

export const GraphCanvas: React.FC<GraphCanvasProps> = ({ nodes }) => {
  return (
    <ReactFlowProvider>
      <GraphCanvasInner nodes={nodes} />
    </ReactFlowProvider>
  );
};
