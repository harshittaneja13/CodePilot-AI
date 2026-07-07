import { useEffect, useId, useState } from 'react';
import mermaid from 'mermaid';

// Initialize once, themed to match the dark UI.
mermaid.initialize({
  startOnLoad: false,
  securityLevel: 'loose',
  theme: 'dark',
  fontFamily: 'Inter, sans-serif',
  themeVariables: {
    background: 'transparent',
    primaryColor: '#1e293b',
    primaryBorderColor: '#334155',
    primaryTextColor: '#e2e8f0',
    secondaryColor: '#0f172a',
    tertiaryColor: '#1e293b',
    lineColor: '#64748b',
    fontSize: '14px',
  },
});

interface MermaidDiagramProps {
  chart: string;
}

/**
 * MermaidDiagram renders a Mermaid definition string to inline SVG. Rendering is async
 * (mermaid v11) and errors are contained so a bad diagram never crashes the page.
 */
export default function MermaidDiagram({ chart }: MermaidDiagramProps) {
  // useId gives a unique, stable id per instance; strip colons for a valid DOM id.
  const id = 'mmd-' + useId().replace(/:/g, '');
  const [svg, setSvg] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    mermaid
      .render(id, chart)
      .then(({ svg }) => {
        if (!cancelled) {
          setSvg(svg);
          setError(null);
        }
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [chart, id]);

  if (error) {
    return (
      <pre className="text-xs text-red-400 font-mono whitespace-pre-wrap p-3">
        Diagram failed to render: {error}
      </pre>
    );
  }

  return (
    <div
      className="overflow-x-auto [&_svg]:max-w-none [&_svg]:mx-auto"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
