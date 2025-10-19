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
import { WorkNodeComponent } from './WorkNodeGraphComponent';

interface GraphCanvasProps {
  className?: string;
  nodes: Map<string, NodeData>;
}

// Register custom node type (outside component to prevent recreation)
const nodeTypes: NodeTypes = {
  workNode: WorkNodeComponent,
};

const GraphCanvasInner: React.FC<GraphCanvasProps> = ({ className, nodes }) => {
  const reactFlowInstance = useReactFlow();
  const hasInitialized = useRef(false);

  // Create a stable key based only on node IDs (for layout recalculation)
  const nodeIdsKey = useMemo(() => {
    return Array.from(nodes.keys()).sort().join(',');
  }, [nodes]);

  // Recalculate layout when nodes are added/removed
  const layoutedElements = useMemo(() => {
    return getLayoutedElements(nodes);
  }, [nodeIdsKey]);

  const [nodesState, setNodes, onNodesChange] = useNodesState(layoutedElements.nodes);
  const [edgesState, setEdges, onEdgesChange] = useEdgesState(layoutedElements.edges);

  // Update layout when nodes are added/removed
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

  // Update node data when statuses change (without recalculating layout)
  useEffect(() => {
    setNodes((currentNodes) =>
      currentNodes.map((node) => {
        const updatedData = nodes.get(node.id);
        if (updatedData) {
          return { ...node, data: updatedData };
        }
        return node;
      })
    );
  }, [nodes, setNodes]);

  return (
    <div className={`${styles.graphContainer} ${className}`}>
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
