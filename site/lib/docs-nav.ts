export interface NavItem {
  title: string;
  href: string;
}

export interface NavSection {
  title: string;
  items: NavItem[];
}

export const docsNav: NavSection[] = [
  {
    title: "Getting Started",
    items: [
      { title: "Overview", href: "/docs" },
      { title: "Quickstart", href: "/docs/quickstart" },
      { title: "Concepts", href: "/docs/concepts" },
    ],
  },
  {
    title: "Configuration",
    items: [
      { title: "Config Methods", href: "/docs/configuration" },
      { title: "CLI Reference", href: "/docs/cli" },
    ],
  },
  {
    title: "Core Concepts",
    items: [
      { title: "Workspaces", href: "/docs/workspaces" },
      { title: "Downstream Servers", href: "/docs/downstream-servers" },
      { title: "Routing Engine", href: "/docs/routing" },
      { title: "Authentication", href: "/docs/authentication" },
      { title: "Secrets", href: "/docs/secrets" },
    ],
  },
  {
    title: "Features",
    items: [
      { title: "Tool Approvals", href: "/docs/approvals" },
      { title: "Audit Logging", href: "/docs/audit" },
      { title: "Caching", href: "/docs/caching" },
      { title: "GitHub Policy", href: "/docs/github-policy" },
      { title: "Connect / Bridge", href: "/docs/connect" },
    ],
  },
  {
    title: "Operations",
    items: [
      { title: "Dashboard & Web UI", href: "/docs/dashboard" },
      { title: "REST API Reference", href: "/docs/api" },
      { title: "Control Server", href: "/docs/control-server" },
      { title: "Desktop App", href: "/docs/desktop-app" },
      { title: "Deployment", href: "/docs/deployment" },
      { title: "Troubleshooting", href: "/docs/troubleshooting" },
    ],
  },
];

/** Flat list of all doc pages in order */
export const allDocPages: NavItem[] = docsNav.flatMap((s) => s.items);

/** Get prev/next pages for a given path */
export function getPrevNext(pathname: string): {
  prev: NavItem | null;
  next: NavItem | null;
} {
  const idx = allDocPages.findIndex((p) => p.href === pathname);
  return {
    prev: idx > 0 ? allDocPages[idx - 1] : null,
    next: idx < allDocPages.length - 1 ? allDocPages[idx + 1] : null,
  };
}
