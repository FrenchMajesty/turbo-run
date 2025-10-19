import { create } from 'zustand';
import { NodeData } from '../types/NodeData';

interface NodesState {
  nodes: Map<string, NodeData>;
  addNode: (nodeId: string, nodeData: NodeData) => void;
  updateNode: (nodeId: string, updates: Partial<NodeData>) => void;
  deleteNode: (nodeId: string) => void;
  clearNodes: () => void;
  getNode: (nodeId: string) => NodeData | undefined;
}

export const useNodesStore = create<NodesState>((set, get) => ({
  nodes: new Map(),

  addNode: (nodeId, nodeData) =>
    set((state) => {
      const newNodes = new Map(state.nodes);
      newNodes.set(nodeId, nodeData);
      return { nodes: newNodes };
    }),

  updateNode: (nodeId, updates) =>
    set((state) => {
      const newNodes = new Map(state.nodes);
      const existingNode = newNodes.get(nodeId);

      if (existingNode) {
        newNodes.set(nodeId, {
          ...existingNode,
          ...updates,
          data: { ...existingNode.data, ...updates.data },
        });
      }

      return { nodes: newNodes };
    }),

  deleteNode: (nodeId) =>
    set((state) => {
      const newNodes = new Map(state.nodes);
      newNodes.delete(nodeId);
      return { nodes: newNodes };
    }),

  clearNodes: () => set({ nodes: new Map() }),

  getNode: (nodeId) => get().nodes.get(nodeId),
}));
