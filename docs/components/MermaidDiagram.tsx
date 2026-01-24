'use client'

import { useEffect, useRef, useState } from 'react'

interface MermaidDiagramProps {
  chart: string
}

export default function MermaidDiagram({ chart }: MermaidDiagramProps) {
  const ref = useRef<HTMLDivElement>(null)
  const [svg, setSvg] = useState('')

  useEffect(() => {
    import('mermaid').then((mermaid) => {
      mermaid.default.initialize({
        startOnLoad: false,
        theme: 'neutral',
        themeVariables: {
          primaryColor: '#4a90d9',
          primaryTextColor: '#fff',
          primaryBorderColor: '#3a7bc8',
          lineColor: '#4a90d9',
          secondaryColor: '#eef5fc',
          tertiaryColor: '#f8fafc',
        },
      })
      const id = `mermaid-${Math.random().toString(36).slice(2)}`
      mermaid.default.render(id, chart).then(({ svg }) => {
        setSvg(svg)
      })
    })
  }, [chart])

  return (
    <div
      ref={ref}
      className="my-6 flex justify-center"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  )
}
