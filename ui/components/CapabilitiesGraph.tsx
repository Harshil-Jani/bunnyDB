'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { useTheme } from 'next-themes';
import { Zap, Table, RefreshCw, GitBranch, Shield, Activity, LucideIcon } from 'lucide-react';
import { BunnyLogo } from './BunnyLogo';
import * as d3 from 'd3';

interface Feature {
  id: string;
  icon: LucideIcon;
  title: string;
  description: string;
  color: string;
}

const features: Feature[] = [
  {
    id: 'cdc',
    icon: Zap,
    title: 'Real-time CDC',
    description: 'Logical replication via WAL decoding. Changes propagate in under a second with zero polling overhead.',
    color: '#eab308',
  },
  {
    id: 'tables',
    icon: Table,
    title: 'Table-level Control',
    description: 'Select exactly which tables to replicate. Add or remove tables on-the-fly while the mirror is paused.',
    color: '#3b82f6',
  },
  {
    id: 'snapshot',
    icon: RefreshCw,
    title: 'Initial Snapshot',
    description: 'Parallel snapshot with configurable partitioning. Bulk-copy existing data before switching to CDC.',
    color: '#22c55e',
  },
  {
    id: 'schema',
    icon: GitBranch,
    title: 'Schema Replication',
    description: 'Indexes, foreign keys, and DDL changes sync automatically. One-click schema sync when you need it.',
    color: '#a855f7',
  },
  {
    id: 'fault',
    icon: Shield,
    title: 'Fault Tolerant',
    description: 'Automatic retries with exponential backoff. Pause, resume, or force-resync any table independently.',
    color: '#ef4444',
  },
  {
    id: 'observe',
    icon: Activity,
    title: 'Full Observability',
    description: 'Per-table row counts, LSN tracking, sync batch IDs, and structured logs — all in real-time.',
    color: '#f0750f',
  },
];

interface NodePosition {
  x: number;
  y: number;
}

interface Particle {
  edgeIndex: number;
  progress: number;
  speed: number;
}

export function CapabilitiesGraph() {
  const containerRef = useRef<HTMLDivElement>(null);
  const animationRef = useRef<number>(0);
  const { resolvedTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  const [dimensions, setDimensions] = useState({ width: 800, height: 600 });
  const [nodePositions, setNodePositions] = useState<NodePosition[]>([]);
  const [hubPosition, setHubPosition] = useState<NodePosition>({ x: 400, y: 300 });
  const [hoveredNode, setHoveredNode] = useState<number | null>(null);
  const [selectedNode, setSelectedNode] = useState<number | null>(null);
  const [particles, setParticles] = useState<Particle[]>([]);
  const [isMobile, setIsMobile] = useState(false);
  const [simulationDone, setSimulationDone] = useState(false);

  const isDark = resolvedTheme === 'dark';

  // Initialize dimensions and detect mobile
  useEffect(() => {
    setMounted(true);
    const updateDimensions = () => {
      if (containerRef.current) {
        const width = containerRef.current.clientWidth;
        const mobile = width < 640;
        setIsMobile(mobile);
        const height = mobile ? 400 : Math.min(600, width * 0.7);
        setDimensions({ width, height });
      }
    };

    updateDimensions();
    const observer = new ResizeObserver(updateDimensions);
    if (containerRef.current) observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, []);

  // Run D3 force simulation
  useEffect(() => {
    if (!mounted || isMobile) return;

    const { width, height } = dimensions;
    const centerX = width / 2;
    const centerY = height / 2;
    const radius = Math.min(width, height) * 0.32;

    // Create nodes: hub (index 0) + 6 features
    const nodes = [
      { x: centerX, y: centerY, fx: centerX, fy: centerY }, // hub is fixed
      ...features.map((_, i) => {
        const angle = (i / features.length) * 2 * Math.PI - Math.PI / 2;
        return {
          x: centerX + radius * Math.cos(angle),
          y: centerY + radius * Math.sin(angle),
        };
      }),
    ];

    // Create links from hub to each feature
    const links = features.map((_, i) => ({ source: 0, target: i + 1 }));

    const simulation = d3
      .forceSimulation(nodes as d3.SimulationNodeDatum[])
      .force('link', d3.forceLink(links).distance(radius).strength(0.3))
      .force('charge', d3.forceManyBody().strength(-200))
      .force('center', d3.forceCenter(centerX, centerY))
      .force('collision', d3.forceCollide(50))
      .alpha(0.8)
      .alphaDecay(0.02);

    simulation.on('tick', () => {
      setHubPosition({ x: nodes[0].x!, y: nodes[0].y! });
      setNodePositions(
        nodes.slice(1).map((n) => ({
          x: Math.max(60, Math.min(width - 60, n.x!)),
          y: Math.max(60, Math.min(height - 60, n.y!)),
        }))
      );
    });

    simulation.on('end', () => {
      setSimulationDone(true);
    });

    // Initialize particles
    const initialParticles: Particle[] = [];
    for (let i = 0; i < features.length; i++) {
      // 2 particles per edge, staggered
      initialParticles.push({ edgeIndex: i, progress: Math.random(), speed: 0.003 + Math.random() * 0.002 });
      initialParticles.push({ edgeIndex: i, progress: Math.random(), speed: 0.002 + Math.random() * 0.002 });
    }
    setParticles(initialParticles);

    return () => {
      simulation.stop();
    };
  }, [mounted, dimensions, isMobile]);

  // Particle animation loop
  useEffect(() => {
    if (!mounted || isMobile || nodePositions.length === 0) return;

    const animate = () => {
      setParticles((prev) =>
        prev.map((p) => {
          const isHovered = hoveredNode === p.edgeIndex;
          const speed = isHovered ? p.speed * 3 : p.speed;
          let progress = p.progress + speed;
          if (progress > 1) progress -= 1;
          return { ...p, progress };
        })
      );
      animationRef.current = requestAnimationFrame(animate);
    };

    animationRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animationRef.current);
  }, [mounted, isMobile, nodePositions.length, hoveredNode]);

  const getParticlePosition = useCallback(
    (particle: Particle) => {
      if (nodePositions.length === 0) return { x: 0, y: 0 };
      const node = nodePositions[particle.edgeIndex];
      if (!node) return { x: 0, y: 0 };
      // Interpolate from hub to node
      const x = hubPosition.x + (node.x - hubPosition.x) * particle.progress;
      const y = hubPosition.y + (node.y - hubPosition.y) * particle.progress;
      return { x, y };
    },
    [nodePositions, hubPosition]
  );

  const handleNodeClick = (index: number) => {
    setSelectedNode(selectedNode === index ? null : index);
  };

  if (!mounted) {
    return <div ref={containerRef} className="w-full h-[600px]" />;
  }

  // Mobile: animated list fallback
  if (isMobile) {
    return (
      <div ref={containerRef} className="w-full space-y-4">
        {features.map((feature, i) => {
          const Icon = feature.icon;
          return (
            <div
              key={feature.id}
              className="flex items-start gap-4 p-4 rounded-xl bg-white dark:bg-gray-800/50 border border-gray-100 dark:border-gray-700/50 shadow-sm transition-all duration-500"
              style={{
                opacity: mounted ? 1 : 0,
                transform: mounted ? 'translateX(0)' : 'translateX(-20px)',
                transitionDelay: `${i * 100}ms`,
                borderLeftWidth: '4px',
                borderLeftColor: feature.color,
              }}
            >
              <div
                className="flex-shrink-0 p-2 rounded-lg"
                style={{ backgroundColor: `${feature.color}20` }}
              >
                <Icon className="w-5 h-5" style={{ color: feature.color }} />
              </div>
              <div>
                <h4 className="font-semibold text-gray-900 dark:text-white">{feature.title}</h4>
                <p className="mt-1 text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                  {feature.description}
                </p>
              </div>
            </div>
          );
        })}
      </div>
    );
  }

  // Desktop: interactive force graph
  return (
    <div ref={containerRef} className="w-full relative">
      <svg
        width={dimensions.width}
        height={dimensions.height}
        className="w-full"
        style={{ height: dimensions.height }}
      >
        {/* Background subtle glow */}
        <defs>
          {features.map((feature, i) => (
            <radialGradient key={`glow-${feature.id}`} id={`glow-${feature.id}`}>
              <stop offset="0%" stopColor={feature.color} stopOpacity="0.3" />
              <stop offset="100%" stopColor={feature.color} stopOpacity="0" />
            </radialGradient>
          ))}
          <radialGradient id="hub-glow">
            <stop offset="0%" stopColor="#f0750f" stopOpacity="0.2" />
            <stop offset="100%" stopColor="#f0750f" stopOpacity="0" />
          </radialGradient>
        </defs>

        {/* Edges */}
        {nodePositions.map((node, i) => {
          const isHighlighted = hoveredNode === i || selectedNode === i;
          return (
            <line
              key={`edge-${i}`}
              x1={hubPosition.x}
              y1={hubPosition.y}
              x2={node.x}
              y2={node.y}
              stroke={isHighlighted ? features[i].color : isDark ? '#374151' : '#e5e7eb'}
              strokeWidth={isHighlighted ? 2.5 : 1.5}
              strokeOpacity={isHighlighted ? 0.9 : 0.5}
              strokeDasharray={isHighlighted ? 'none' : '4 4'}
              className="transition-all duration-300"
            />
          );
        })}

        {/* Particles */}
        {particles.map((particle, i) => {
          const pos = getParticlePosition(particle);
          const isHighlighted = hoveredNode === particle.edgeIndex || selectedNode === particle.edgeIndex;
          const color = features[particle.edgeIndex]?.color || '#f0750f';
          return (
            <circle
              key={`particle-${i}`}
              cx={pos.x}
              cy={pos.y}
              r={isHighlighted ? 4 : 2.5}
              fill={color}
              opacity={isHighlighted ? 0.9 : 0.5}
              className="transition-[r,opacity] duration-200"
            />
          );
        })}

        {/* Hub glow */}
        <circle
          cx={hubPosition.x}
          cy={hubPosition.y}
          r={55}
          fill="url(#hub-glow)"
        />

        {/* Hub node */}
        <g className="cursor-pointer">
          <circle
            cx={hubPosition.x}
            cy={hubPosition.y}
            r={38}
            fill={isDark ? '#1f2937' : '#ffffff'}
            stroke="#f0750f"
            strokeWidth={3}
            className="transition-all duration-300"
          />
          <foreignObject
            x={hubPosition.x - 16}
            y={hubPosition.y - 16}
            width={32}
            height={32}
          >
            <div className="w-full h-full flex items-center justify-center">
              <BunnyLogo size={28} />
            </div>
          </foreignObject>
          {/* Pulsing ring */}
          <circle
            cx={hubPosition.x}
            cy={hubPosition.y}
            r={38}
            fill="none"
            stroke="#f0750f"
            strokeWidth={1.5}
            opacity={0.4}
            className="animate-ping"
            style={{ animationDuration: '3s' }}
          />
        </g>

        {/* Feature nodes */}
        {nodePositions.map((node, i) => {
          const feature = features[i];
          const Icon = feature.icon;
          const isHovered = hoveredNode === i;
          const isSelected = selectedNode === i;
          const isActive = isHovered || isSelected;
          const nodeRadius = isActive ? 36 : 30;

          return (
            <g
              key={feature.id}
              className="cursor-pointer transition-transform duration-300"
              onMouseEnter={() => setHoveredNode(i)}
              onMouseLeave={() => setHoveredNode(null)}
              onClick={() => handleNodeClick(i)}
            >
              {/* Node glow on hover */}
              {isActive && (
                <circle
                  cx={node.x}
                  cy={node.y}
                  r={nodeRadius + 20}
                  fill={`url(#glow-${feature.id})`}
                  className="transition-all duration-300"
                />
              )}

              {/* Node circle */}
              <circle
                cx={node.x}
                cy={node.y}
                r={nodeRadius}
                fill={isDark ? '#1f2937' : '#ffffff'}
                stroke={feature.color}
                strokeWidth={isActive ? 3 : 2}
                className="transition-all duration-300"
                style={{
                  filter: isActive ? `drop-shadow(0 0 8px ${feature.color}60)` : 'none',
                }}
              />

              {/* Icon */}
              <foreignObject
                x={node.x - 12}
                y={node.y - 12}
                width={24}
                height={24}
              >
                <div className="w-full h-full flex items-center justify-center">
                  <Icon
                    className="w-5 h-5 transition-transform duration-300"
                    style={{
                      color: feature.color,
                      transform: isActive ? 'scale(1.2)' : 'scale(1)',
                    }}
                  />
                </div>
              </foreignObject>

              {/* Label */}
              <text
                x={node.x}
                y={node.y + nodeRadius + 18}
                textAnchor="middle"
                className="text-xs font-semibold transition-all duration-300 select-none"
                fill={isActive ? feature.color : isDark ? '#d1d5db' : '#4b5563'}
              >
                {feature.title}
              </text>

              {/* Hover tooltip */}
              {isActive && (
                <foreignObject
                  x={node.x - 140}
                  y={node.y - nodeRadius - 90}
                  width={280}
                  height={80}
                >
                  <div
                    className="px-3 py-2.5 rounded-lg shadow-lg text-center text-xs leading-relaxed"
                    style={{
                      backgroundColor: isDark ? '#1f2937' : '#ffffff',
                      color: isDark ? '#d1d5db' : '#4b5563',
                      border: `1px solid ${isDark ? '#374151' : '#e5e7eb'}`,
                    }}
                  >
                    {feature.description}
                  </div>
                </foreignObject>
              )}
            </g>
          );
        })}
      </svg>

      {/* Selected feature detail panel */}
      {selectedNode !== null && (
        <div
          className="mt-4 p-6 rounded-2xl border transition-all duration-300 animate-in fade-in slide-in-from-bottom-4"
          style={{
            backgroundColor: isDark ? '#111827' : '#ffffff',
            borderColor: features[selectedNode].color + '40',
            boxShadow: `0 0 20px ${features[selectedNode].color}10`,
          }}
        >
          <div className="flex items-center gap-3">
            <div
              className="p-3 rounded-xl"
              style={{ backgroundColor: `${features[selectedNode].color}15` }}
            >
              {(() => {
                const Icon = features[selectedNode].icon;
                return <Icon className="w-6 h-6" style={{ color: features[selectedNode].color }} />;
              })()}
            </div>
            <div>
              <h4 className="text-lg font-bold text-gray-900 dark:text-white">
                {features[selectedNode].title}
              </h4>
              <p className="text-gray-600 dark:text-gray-400 leading-relaxed">
                {features[selectedNode].description}
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
