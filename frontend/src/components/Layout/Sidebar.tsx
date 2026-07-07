import { NavLink, useLocation } from 'react-router-dom';
import {
  LayoutDashboard,
  GitFork,
  MessageSquareCode,
  Bot,
  ListChecks,
  Workflow,
} from 'lucide-react';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/repositories', icon: GitFork, label: 'Repositories' },
  { to: '/pull-requests', icon: ListChecks, label: 'Pull Requests' },
  { to: '/reviews', icon: MessageSquareCode, label: 'Reviews' },
  { to: '/pipeline', icon: Workflow, label: 'Pipeline' },
];

export default function Sidebar() {
  const location = useLocation();

  return (
    <aside className="fixed left-0 top-0 z-50 h-screen w-[260px] bg-[rgba(15,23,42,0.95)] backdrop-blur-xl border-r border-[rgba(51,65,85,0.5)] flex flex-col">
      {/* Logo */}
      <div className="px-6 py-6 border-b border-[rgba(51,65,85,0.3)]">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-[#3b82f6] to-[#8b5cf6] flex items-center justify-center shadow-lg shadow-blue-500/25">
            <Bot className="w-5 h-5 text-white" />
          </div>
          <div>
            <h1 className="text-lg font-bold bg-gradient-to-r from-[#3b82f6] to-[#8b5cf6] bg-clip-text text-transparent">
              CodePilot AI
            </h1>
            <p className="text-[10px] text-[#64748b] font-medium tracking-wider uppercase">
              PR Review Agent
            </p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-6 space-y-1">
        {navItems.map(({ to, icon: Icon, label }) => {
          const isActive =
            to === '/'
              ? location.pathname === '/'
              : location.pathname.startsWith(to);
          return (
            <NavLink
              key={to}
              to={to}
              className={`
                group flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-medium
                transition-all duration-200 relative
                ${
                  isActive
                    ? 'bg-gradient-to-r from-[#3b82f6]/20 to-[#3b82f6]/5 text-[#60a5fa] shadow-lg shadow-blue-500/10'
                    : 'text-[#94a3b8] hover:text-[#f8fafc] hover:bg-[#1e293b]/60 hover:translate-x-1'
                }
              `}
            >
              {isActive && (
                <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-8 bg-[#3b82f6] rounded-r-full shadow-lg shadow-blue-500/50" />
              )}
              <Icon
                className={`w-5 h-5 transition-all duration-200 ${
                  isActive
                    ? 'text-[#3b82f6]'
                    : 'text-[#64748b] group-hover:text-[#94a3b8]'
                }`}
              />
              {label}
            </NavLink>
          );
        })}
      </nav>

      {/* Bottom */}
      <div className="px-6 py-4 border-t border-[rgba(51,65,85,0.3)]">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-[#10b981] shadow-lg shadow-emerald-500/50 animate-pulse" />
            <span className="text-xs text-[#64748b]">Connected</span>
          </div>
          <span className="text-[10px] text-[#475569] font-mono">v1.0.0</span>
        </div>
      </div>
    </aside>
  );
}
