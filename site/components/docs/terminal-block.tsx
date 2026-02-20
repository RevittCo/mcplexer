interface TerminalBlockProps {
  title?: string;
  children: React.ReactNode;
}

export function TerminalBlock({
  title = "terminal",
  children,
}: TerminalBlockProps) {
  return (
    <div className="terminal rounded overflow-hidden mb-4">
      <div className="terminal-header">
        <span className="terminal-dot bg-red/80" />
        <span className="terminal-dot bg-amber/80" />
        <span className="terminal-dot bg-green/80" />
        <span className="ml-2 text-[10px] text-text-dim">{title}</span>
      </div>
      <div className="p-4 text-sm leading-relaxed font-mono whitespace-pre-wrap text-text-muted">
        {children}
      </div>
    </div>
  );
}
