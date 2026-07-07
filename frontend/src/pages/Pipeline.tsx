import MermaidDiagram from '@/components/common/MermaidDiagram';
import { pipelineDiagrams } from '@/lib/pipelineDiagrams';

const agentTools: { tool: string; when: string; impl: string; rag?: boolean }[] = [
  { tool: 'get_file_diff', when: "Inspect a changed file's diff", impl: 'PR patch map (no I/O)' },
  { tool: 'get_file_contents', when: 'See full surrounding code at PR head', impl: 'GitHub MCP' },
  { tool: 'search_code', when: 'Find where a symbol is defined / used', impl: 'GitHub MCP' },
  { tool: 'retrieve_context', when: 'Semantic cross-file context beyond the diff', impl: 'RAG · Qdrant — only when RAG enabled', rag: true },
  { tool: 'submit_findings', when: 'Terminal — end the loop with the review', impl: 'Parsed into ReviewResult' },
];

const gates: { gate: string; cond: string }[] = [
  { gate: 'Triage vs review-all', cond: 'changed files ≥ 4 (triageSkipThreshold)' },
  { gate: 'Agent vs fixed', cond: 'model supports tools AND agent succeeds' },
  { gate: 'RAG on / off', cond: 'RAG_ENABLED AND retriever ready' },
  { gate: 'Agent stop', cond: 'submit_findings called, or maxSteps (8)' },
  { gate: 'Finding kept', cond: 'confidence ≥ 0.7 AND is_valid' },
  { gate: 'Review event', cond: 'any critical finding → REQUEST_CHANGES, else COMMENT' },
];

export default function Pipeline() {
  return (
    <div className="space-y-8 max-w-6xl">
      {/* Header */}
      <div className="opacity-0 animate-fadeIn">
        <h1 className="text-2xl font-bold text-[#f8fafc]">AI Pipeline</h1>
        <p className="text-[#94a3b8] text-sm mt-1">
          How the review agent works — phases, the tools it calls, and when it uses RAG.
        </p>
      </div>

      {/* Diagrams */}
      {pipelineDiagrams.map((d, i) => (
        <div
          key={d.title}
          className="glass-card p-6 opacity-0 animate-fadeIn"
          style={{ animationDelay: `${i * 80}ms` }}
        >
          <h2 className="text-base font-semibold text-[#f8fafc]">{d.title}</h2>
          <p className="text-xs text-[#94a3b8] mt-1 mb-4">{d.description}</p>
          <MermaidDiagram chart={d.chart} />
        </div>
      ))}

      {/* Agent tool inventory */}
      <div className="glass-card overflow-hidden">
        <div className="px-6 py-4 border-b border-[rgba(51,65,85,0.5)]">
          <h2 className="text-base font-semibold text-[#f8fafc]">Agent tool inventory</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-[rgba(51,65,85,0.3)]">
                <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">Tool</th>
                <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">When the model calls it</th>
                <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">Backing implementation</th>
              </tr>
            </thead>
            <tbody>
              {agentTools.map((t) => (
                <tr key={t.tool} className="border-b border-[rgba(51,65,85,0.2)]">
                  <td className="px-6 py-3">
                    <span className={`font-mono text-xs ${t.rag ? 'text-emerald-400' : 'text-[#60a5fa]'}`}>
                      {t.tool}
                    </span>
                  </td>
                  <td className="px-6 py-3 text-sm text-[#e2e8f0]">{t.when}</td>
                  <td className="px-6 py-3 text-xs text-[#94a3b8]">{t.impl}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Decision gates */}
      <div className="glass-card overflow-hidden">
        <div className="px-6 py-4 border-b border-[rgba(51,65,85,0.5)]">
          <h2 className="text-base font-semibold text-[#f8fafc]">Decision gates (the “when”)</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-[rgba(51,65,85,0.3)]">
                <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">Gate</th>
                <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">Condition</th>
              </tr>
            </thead>
            <tbody>
              {gates.map((g) => (
                <tr key={g.gate} className="border-b border-[rgba(51,65,85,0.2)]">
                  <td className="px-6 py-3 text-sm font-medium text-[#e2e8f0]">{g.gate}</td>
                  <td className="px-6 py-3 text-sm text-[#94a3b8]">{g.cond}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <p className="text-xs text-[#475569]">
        Full written blueprint: <span className="font-mono">docs/AI_PIPELINE.md</span>
      </p>
    </div>
  );
}
