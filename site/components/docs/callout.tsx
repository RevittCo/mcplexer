import { AlertTriangle, Info, Lightbulb, ShieldAlert } from "lucide-react";

const variants = {
  info: {
    icon: Info,
    border: "border-cyan/40",
    bg: "bg-cyan/5",
    iconColor: "text-cyan",
  },
  warning: {
    icon: AlertTriangle,
    border: "border-amber/40",
    bg: "bg-amber/5",
    iconColor: "text-amber",
  },
  tip: {
    icon: Lightbulb,
    border: "border-green/40",
    bg: "bg-green/5",
    iconColor: "text-green",
  },
  danger: {
    icon: ShieldAlert,
    border: "border-red/40",
    bg: "bg-red/5",
    iconColor: "text-red",
  },
} as const;

interface CalloutProps {
  type?: keyof typeof variants;
  title?: string;
  children: React.ReactNode;
}

export function Callout({ type = "info", title, children }: CalloutProps) {
  const v = variants[type];
  const Icon = v.icon;

  return (
    <div
      className={`flex gap-3 p-4 mb-4 border-l-2 ${v.border} ${v.bg} rounded-r`}
    >
      <Icon className={`w-4 h-4 mt-0.5 shrink-0 ${v.iconColor}`} />
      <div className="text-sm text-text-muted">
        {title && (
          <p className="font-semibold text-text mb-1">{title}</p>
        )}
        {children}
      </div>
    </div>
  );
}
