interface Prop {
  name: string;
  type: string;
  default?: string;
  description: string;
}

interface PropTableProps {
  props: Prop[];
}

export function PropTable({ props }: PropTableProps) {
  return (
    <div className="overflow-x-auto mb-4">
      <table className="w-full text-sm border border-border">
        <thead className="bg-surface text-text-muted text-xs uppercase tracking-wider">
          <tr>
            <th className="text-left px-3 py-2 border-b border-border font-medium">
              Name
            </th>
            <th className="text-left px-3 py-2 border-b border-border font-medium">
              Type
            </th>
            <th className="text-left px-3 py-2 border-b border-border font-medium">
              Default
            </th>
            <th className="text-left px-3 py-2 border-b border-border font-medium">
              Description
            </th>
          </tr>
        </thead>
        <tbody>
          {props.map((prop) => (
            <tr key={prop.name} className="hover:bg-surface/50">
              <td className="px-3 py-2 border-b border-border">
                <code className="px-1 py-0.5 bg-surface text-cyan text-xs border border-border rounded">
                  {prop.name}
                </code>
              </td>
              <td className="px-3 py-2 border-b border-border text-text-dim text-xs">
                {prop.type}
              </td>
              <td className="px-3 py-2 border-b border-border text-text-dim text-xs">
                {prop.default || "—"}
              </td>
              <td className="px-3 py-2 border-b border-border text-text-muted text-xs">
                {prop.description}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
