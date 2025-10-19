import dagre from '@dagrejs/dagre';
import { Node, Edge, Position } from '@xyflow/react';
import { NodeData } from '../types/NodeData';

const NODE_WIDTH = 97;
const NODE_HEIGHT = 75;

export interface LayoutedNode extends Node<NodeData> {
  position: { x: number; y: number };
}

export const getLayoutedElements = (
  nodesMap: Map<string, NodeData>
): { nodes: LayoutedNode[]; edges: Edge[] } => {
  const dagreGraph = new dagre.graphlib.Graph();
  dagreGraph.setDefaultEdgeLabel(() => ({}));

  // Configure graph layout
  dagreGraph.setGraph({
    rankdir: 'LR', // Left to Right (horizontal flow)
    nodesep: 50,   // Vertical spacing between nodes in same rank
    ranksep: 100,  // Horizontal spacing between ranks
    marginx: 20,
    marginy: 20,
  });

  const nodes: LayoutedNode[] = [];
  const edges: Edge[] = [];

  // Add all nodes to dagre graph
  nodesMap.forEach((nodeData, nodeId) => {
    dagreGraph.setNode(nodeId, { width: NODE_WIDTH, height: NODE_HEIGHT });

    // Create React Flow node
    nodes.push({
      id: nodeId,
      type: 'workNode',
      data: nodeData,
      position: { x: 0, y: 0 }, // Will be set by dagre
      sourcePosition: Position.Right,  // For LR: arrows exit from right
      targetPosition: Position.Left,   // For LR: arrows enter from left
    });
  });

  // Add edges based on dependencies
  nodesMap.forEach((nodeData, nodeId) => {
    if (nodeData.dependencies && nodeData.dependencies.length > 0) {
      nodeData.dependencies.forEach((depId) => {
        // Only create edge if both nodes exist
        if (nodesMap.has(depId)) {
          // Edge from dependency to dependent node
          dagreGraph.setEdge(depId, nodeId);

          edges.push({
            id: `${depId}-${nodeId}`,
            source: depId,
            target: nodeId,
            type: 'smoothstep',
            animated: nodeData.status === 'node_running',
          });
        }
      });
    }
  });

  // Calculate layout
  dagre.layout(dagreGraph);

  // Apply calculated positions to nodes
  nodes.forEach((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    node.position = {
      x: nodeWithPosition.x - NODE_WIDTH / 2,
      y: nodeWithPosition.y - NODE_HEIGHT / 2,
    };
  });

  return { nodes, edges };
};
