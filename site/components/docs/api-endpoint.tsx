interface ApiEndpointProps {
  method: "GET" | "POST" | "PUT" | "DELETE";
  path: string;
  description: string;
  children?: React.ReactNode;
}

const methodColors = {
  GET: "bg-green/20 text-green border-green/30",
  POST: "bg-cyan/20 text-cyan border-cyan/30",
  PUT: "bg-amber/20 text-amber border-amber/30",
  DELETE: "bg-red/20 text-red border-red/30",
} as const;

export function ApiEndpoint({
  method,
  path,
  description,
  children,
}: ApiEndpointProps) {
  return (
    <div className="mb-4 border border-border rounded overflow-hidden">
      <div className="flex items-center gap-3 px-3 py-2 bg-surface">
        <span
          className={`px-2 py-0.5 text-[10px] font-bold uppercase border rounded ${methodColors[method]}`}
        >
          {method}
        </span>
        <code className="text-xs text-text">{path}</code>
      </div>
      <div className="px-3 py-2 text-xs text-text-muted">{description}</div>
      {children && (
        <div className="px-3 py-2 border-t border-border text-xs text-text-muted">
          {children}
        </div>
      )}
    </div>
  );
}
