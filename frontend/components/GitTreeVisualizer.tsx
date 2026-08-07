'use client';

import { useCallback, useEffect, useState } from 'react';
import ReactFlow, {
  Background,
  Controls,
  Edge,
  Node,
  Position,
  Handle,
  MarkerType,
} from 'reactflow';
import 'reactflow/dist/style.css';
import dagre from 'dagre';
import { fetchWithAuth } from '@/lib/auth';

// Tooltip Component
const CommitTooltip = ({ data, colorClass, borderClass, textClass, bgClass }: any) => (
  <div className={`absolute bottom-full left-1/2 -translate-x-1/2 mb-3 w-48 p-2.5 rounded-lg bg-surface-container-highest border shadow-xl opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-50 text-left ${borderClass}`}>
    <div className={`font-mono text-[10px] mb-0.5 ${textClass}`}>{data.hash}</div>
    <div className="font-bold text-xs text-on-surface line-clamp-2" title={data.label}>{data.label}</div>
    <div className={`text-[10px] mt-1 flex justify-between items-center ${textClass}`}>
      <span className="truncate mr-2 opacity-80">{data.author}</span>
      {data.refs && <span className={`${bgClass} px-1.5 py-0.5 rounded truncate max-w-[80px] font-medium`} title={data.refs}>{data.refs}</span>}
    </div>
  </div>
);

const GithubCommitNode = ({ data }: { data: any }) => (
  <div className="group relative flex items-center justify-center w-6 h-6">
    <div className="w-4 h-4 rounded-full bg-teal-600 border border-teal-300 shadow-[0_0_10px_#2dd4bf] node-shine"></div>
    <Handle type="target" position={Position.Left} className="!w-1 !h-1 !opacity-0" />
    <Handle type="source" position={Position.Right} className="!w-1 !h-1 !opacity-0" />
    <CommitTooltip 
      data={data} 
      borderClass="border-teal-500/30" 
      textClass="text-teal-300" 
      bgClass="bg-teal-900/50" 
    />
  </div>
);

const LocalCommitNode = ({ data }: { data: any }) => (
  <div className="group relative flex items-center justify-center w-6 h-6">
    <div className="w-4 h-4 rounded-full bg-cyan-600 border border-cyan-300 shadow-[0_0_10px_#22d3ee] node-shine"></div>
    <Handle type="target" position={Position.Left} className="!w-1 !h-1 !opacity-0" />
    <Handle type="source" position={Position.Right} className="!w-1 !h-1 !opacity-0" />
    <CommitTooltip 
      data={data} 
      borderClass="border-cyan-500/30" 
      textClass="text-cyan-300" 
      bgClass="bg-cyan-900/50" 
    />
  </div>
);

const LocalEditsNode = ({ data }: { data: any }) => (
  <div className="group relative flex items-center justify-center w-6 h-6">
    <div className="w-4 h-4 rounded-full bg-cyan-900/50 border-2 border-dashed border-cyan-400 shadow-[0_0_10px_#22d3ee] node-shine"></div>
    <Handle type="target" position={Position.Left} className="!w-1 !h-1 !opacity-0" />
    <Handle type="source" position={Position.Right} className="!w-1 !h-1 !opacity-0" />
    <CommitTooltip 
      data={data} 
      borderClass="border-cyan-400/50 border-dashed" 
      textClass="text-cyan-400" 
      bgClass="bg-transparent" 
    />
  </div>
);

const nodeTypes = {
  githubCommit: GithubCommitNode,
  localCommit: LocalCommitNode,
  localEdits: LocalEditsNode,
};

const getLayoutedElements = (nodes: Node[], edges: Edge[], direction = 'LR') => {
  const dagreGraph = new dagre.graphlib.Graph();
  dagreGraph.setDefaultEdgeLabel(() => ({}));
  
  const isHorizontal = direction === 'LR';
  dagreGraph.setGraph({ rankdir: direction, ranksep: 60, nodesep: 40 });

  nodes.forEach((node) => {
    // Exact dimensions of our uniform dot nodes (24x24)
    dagreGraph.setNode(node.id, { width: 24, height: 24 });
  });

  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });

  dagre.layout(dagreGraph);

  nodes.forEach((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    node.targetPosition = isHorizontal ? Position.Left : Position.Top;
    node.sourcePosition = isHorizontal ? Position.Right : Position.Bottom;

    node.position = {
      x: nodeWithPosition.x - 12,
      y: nodeWithPosition.y - 12,
    };

    return node;
  });

  return { nodes, edges };
};

export default function GitTreeVisualizer({ environmentId }: { environmentId: string }) {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [edges, setEdges] = useState<Edge[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const fetchTree = async () => {
      try {
        const data = await fetchWithAuth(`/api/environments/${environmentId}/git-tree`);
        
        const nodesData = data.nodes || [];
        const edgesData = data.edges || [];

        // Style the edges
        const styledEdges = edgesData.map((e: any) => ({
          ...e,
          type: 'default', // bezier curve instead of vertical+horizontal smoothstep
          animated: true,
          style: { stroke: '#4edea3', strokeWidth: 2 },
          markerEnd: {
            type: MarkerType.ArrowClosed,
            width: 15,
            height: 15,
            color: '#4edea3',
          },
        }));

        const { nodes: layoutedNodes, edges: layoutedEdges } = getLayoutedElements(
          nodesData,
          styledEdges
        );

        setNodes([...layoutedNodes]);
        setEdges([...layoutedEdges]);
      } catch (err: any) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchTree();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [environmentId]);

  if (loading) return <div className="p-8 text-center text-on-surface">Loading visual blueprint...</div>;
  if (error) return <div className="p-8 text-center text-error">Error: {error}</div>;

  return (
    <div style={{ width: '100%', height: '500px' }} className="rounded-xl overflow-hidden border border-surface-variant bg-surface-container-lowest">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        attributionPosition="bottom-right"
        className="dark"
      >
        <Background color="#3b494b" gap={24} />
        <Controls className="bg-surface-container border-surface-variant fill-on-surface" />
      </ReactFlow>
    </div>
  );
}
