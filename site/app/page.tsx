import Link from "next/link";
import { config } from "@/lib/config";
import { McplexerLogo } from "@/components/logo";
import {
  Code,
  HardDrive,
  FolderLock,
  ShieldCheck,
  KeyRound,
  ScrollText,
  Shield,
  CloudOff,
  ArrowRight,
  Layers,
  Activity,
  RefreshCw,
  Lock,
  FlaskConical,
  Terminal,
  Copy,
  Cable,
  Wrench,
} from "lucide-react";

const uspCards = [
  {
    icon: Code,
    title: "Open Source",
    description:
      "MIT licensed. Pure Go, single binary, zero CGO. Build from source or go install. Runs anywhere Go compiles to.",
  },
  {
    icon: FolderLock,
    title: "direnv for MCP",
    description:
      "Workspaces bind to directory trees. In stdio mode, your actual CWD determines which policies apply \u2014 tamper-proof, inherited from the parent process.",
  },
  {
    icon: ShieldCheck,
    title: "Tool Approvals",
    description:
      "Per-route approval requirements. Pending requests stream via SSE to the dashboard. Configurable timeouts, resolution tracking, self-approval prevention.",
  },
  {
    icon: Wrench,
    title: "Self-Configurable via MCP",
    description:
      "Run mcplexer control-server to expose 15 MCP tools. Create workspaces, routes, and servers from Claude or any MCP client. Read-only mode available.",
  },
  {
    icon: KeyRound,
    title: "OAuth 2.0 + PKCE",
    description:
      "Built-in OAuth flows with PKCE. Provider templates for GitHub, Slack, and more. Automatic token refresh. Credentials injected into downstream env vars.",
  },
  {
    icon: ScrollText,
    title: "Full Audit Trail",
    description:
      "Every tool call logged with workspace, matched route, auth scope, latency. Parameter redaction via per-scope hints. SSE streaming and query API.",
  },
];

const moreFeatures = [
  {
    icon: HardDrive,
    title: "Local First",
    description:
      "No cloud, no telemetry. SQLite database, age-encrypted secrets, everything on your machine.",
  },
  {
    icon: Cable,
    title: "Stdio + Unix Socket",
    description:
      "Stdio for single client, Unix socket for multi-client. CWD injection bridge for Claude Desktop.",
  },
  {
    icon: Activity,
    title: "Live Dashboard",
    description:
      "Real-time metrics, session tracking, approval queue, audit stream. Full web UI for management.",
  },
  {
    icon: RefreshCw,
    title: "Process Lifecycle",
    description:
      "Downstream process management with restart policies, idle timeouts, instance pooling per auth scope.",
  },
  {
    icon: Lock,
    title: "age Encryption",
    description:
      "Secrets encrypted at rest with filippo.io/age. Auto-generated keys. Managed via CLI or control server.",
  },
  {
    icon: FlaskConical,
    title: "Dry Run",
    description:
      "Test routing decisions without execution. CLI or API. See exactly which rule matches and why.",
  },
];

const steps = [
  {
    num: "01",
    title: "Install",
    description: `Download the app or go install. Single binary, no dependencies.`,
  },
  {
    num: "02",
    title: "Setup",
    description: `Run ${config.name} setup \u2014 it starts the daemon, configures Claude Desktop, and opens the dashboard. One command.`,
  },
  {
    num: "03",
    title: "Control",
    description:
      "Manage everything from the dashboard, REST API, MCP control server, or YAML config.",
  },
];

const metrics = [
  { icon: Shield, label: "Open Source" },
  { icon: HardDrive, label: "Local First" },
  { icon: CloudOff, label: "Zero Cloud" },
  { icon: Code, label: "Pure Go" },
];

export default function HomePage() {
  return (
    <>
      {/* ── Hero ── */}
      <section className="relative min-h-screen flex flex-col justify-center pt-14 overflow-hidden">
        <div className="absolute inset-0 grid-pattern opacity-30" />
        <div className="absolute inset-0 noise" />
        <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,transparent_0%,var(--color-bg)_75%)]" />

        <div className="relative z-10 mx-auto w-full max-w-6xl px-4 sm:px-6 py-16 sm:py-24">
          {/* Big name */}
          <div className="scanlines mb-10 sm:mb-14">
            <h1
              className="hero-name select-none"
              data-text={config.name}
              aria-label={config.name}
            >
              {config.name}
            </h1>
          </div>

          {/* Tagline + description */}
          <div className="max-w-2xl">
            <p className="text-lg sm:text-xl font-medium text-text mb-3 leading-snug">
              <span className="offset-line">{config.tagline}</span>
            </p>
            <p className="text-sm sm:text-base text-text-muted leading-relaxed">
              {config.description}
            </p>
          </div>

          {/* CTAs */}
          <div className="mt-8 flex flex-col sm:flex-row items-start gap-3">
            <Link
              href={config.github}
              target="_blank"
              rel="noopener noreferrer"
              className="group flex items-center gap-2 bg-cyan px-5 py-2.5 text-sm font-medium text-bg hover:bg-cyan-light transition-colors"
            >
              Get Started
              <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-0.5" />
            </Link>
            <Link
              href={config.github}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 border border-cyan/30 px-5 py-2.5 text-sm font-medium text-cyan hover:bg-cyan/5 transition-colors"
            >
              GitHub
            </Link>
          </div>

          {/* Terminal mockup */}
          <div className="mt-14 sm:mt-16 w-full max-w-3xl">
            <div className="terminal glow-cyan-sm">
              <div className="terminal-header">
                <span className="terminal-dot bg-red/80" />
                <span className="terminal-dot bg-amber/80" />
                <span className="terminal-dot bg-green/80" />
                <span className="ml-3 text-[11px] text-text-dim">
                  ~/{config.name}
                </span>
              </div>
              <div className="p-4 sm:p-6 text-[11px] sm:text-xs leading-6 sm:leading-7 overflow-x-auto">
                <div>
                  <span className="text-text-muted">$ </span>
                  <span className="text-text">
                    {config.name} setup
                  </span>
                </div>
                <div className="mt-2">
                  <span className="text-green">{"✓"}</span>
                  <span className="text-text-muted"> daemon started on </span>
                  <span className="text-cyan">:3333</span>
                </div>
                <div>
                  <span className="text-green">{"✓"}</span>
                  <span className="text-text-muted"> claude desktop config updated</span>
                </div>
                <div>
                  <span className="text-green">{"✓"}</span>
                  <span className="text-text-muted"> dashboard </span>
                  <span className="text-text-dim">{"\u2192"} </span>
                  <span className="text-cyan">http://localhost:3333</span>
                </div>
                <div className="mt-1">
                  <span className="text-text-dim">restart Claude Desktop to connect.</span>
                </div>

                <div className="mt-4 border-t border-border pt-4">
                  <div className="text-text-dim text-[10px] uppercase tracking-wider mb-2">
                    tool call routed through {config.name}
                  </div>
                  <div>
                    <span className="text-amber">[12:34:15]</span>
                    <span className="text-text-muted"> tool call: </span>
                    <span className="text-cyan font-medium">
                      github__create_issue
                    </span>
                  </div>
                  <div className="ml-[4.5rem] sm:ml-[5.5rem]">
                    <span className="text-text-dim">cwd:      </span>
                    <span className="text-text-muted">~/projects/app</span>
                    <span className="text-text-dim">
                      {" "}
                      {"\u2192"} workspace:{" "}
                    </span>
                    <span className="text-cyan">frontend</span>
                  </div>
                  <div className="ml-[4.5rem] sm:ml-[5.5rem]">
                    <span className="text-text-dim">route:    </span>
                    <span className="text-green">allow</span>
                    <span className="text-text-dim"> (rule: </span>
                    <span className="text-text-muted">
                      &quot;github-write&quot;
                    </span>
                    <span className="text-text-dim">, priority: </span>
                    <span className="text-text-muted">10</span>
                    <span className="text-text-dim">)</span>
                  </div>
                  <div className="ml-[4.5rem] sm:ml-[5.5rem]">
                    <span className="text-text-dim">approval: </span>
                    <span className="text-green">approved</span>
                    <span className="text-text-dim"> (2.1s)</span>
                  </div>
                  <div className="ml-[4.5rem] sm:ml-[5.5rem]">
                    <span className="text-text-dim">status:   </span>
                    <span className="text-green">success</span>
                    <span className="text-text-dim"> (284ms)</span>
                  </div>
                  <div className="mt-1">
                    <span className="text-amber">[12:34:15]</span>
                    <span className="text-text-muted"> audit: </span>
                    <span className="text-text-dim">logged </span>
                    <span className="text-cyan">#1847</span>
                    <span className="cursor-blink ml-0.5" />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Metrics Bar ── */}
      <section className="border-y border-border bg-surface">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="flex flex-wrap items-center justify-center gap-6 sm:gap-12 py-6 sm:py-8">
            {metrics.map(({ icon: Icon, label }) => (
              <div
                key={label}
                className="flex items-center gap-2.5 text-sm text-text-muted"
              >
                <Icon className="w-4 h-4 text-cyan" />
                <span>{label}</span>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Core USPs ── */}
      <section className="py-20 sm:py-28">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-14">
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">
              Built for{" "}
              <span className="text-cyan">security-conscious</span> teams
            </h2>
            <p className="mt-4 text-text-muted text-sm max-w-xl mx-auto">
              Full control over what AI tools can do on your machine.
              Directory-scoped policies, human-in-the-loop approvals, complete
              audit trail.
            </p>
          </div>

          <div className="stagger grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {uspCards.map(({ icon: Icon, title, description }) => (
              <div
                key={title}
                className="group bg-surface border border-border p-6 hover:border-border-hover transition-colors"
              >
                <div className="flex items-center gap-3 mb-4">
                  <div className="flex h-9 w-9 items-center justify-center bg-cyan/10 text-cyan border border-cyan/20">
                    <Icon className="w-4 h-4" />
                  </div>
                  <h3 className="font-semibold text-sm">{title}</h3>
                </div>
                <p className="text-xs leading-relaxed text-text-muted">
                  {description}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Directory-Aware Routing ── */}
      <section className="py-20 sm:py-28 border-t border-border">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-14">
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">
              Your <span className="text-cyan">CWD</span> is your security
              context
            </h2>
            <p className="mt-4 text-text-muted text-sm max-w-2xl mx-auto">
              In stdio mode, {config.name} reads your actual working directory
              from the OS {"\u2014"} no client can fake it. The most specific
              matching workspace wins. Route rules evaluate deny-first through
              the workspace ancestor chain.
            </p>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="terminal">
              <div className="terminal-header">
                <span className="terminal-dot bg-red/80" />
                <span className="terminal-dot bg-amber/80" />
                <span className="terminal-dot bg-green/80" />
                <span className="ml-3 text-[11px] text-text-dim">
                  ~/projects/frontend
                </span>
              </div>
              <div className="p-4 sm:p-5 text-[11px] sm:text-xs leading-6 sm:leading-7">
                <div>
                  <span className="text-text-muted">$ </span>
                  <span className="text-text">cd ~/projects/frontend</span>
                </div>
                <div className="mt-3 text-text-dim text-[10px] uppercase tracking-wider mb-2">
                  workspace: <span className="text-cyan">frontend</span> (most
                  specific match)
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-green">allow</span>
                  <span className="text-text-dim">
                    github__*, slack__send_message
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-red">deny</span>
                  <span className="text-text-dim">
                    aws__*, docker__*, *__delete_*
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-amber">approve</span>
                  <span className="text-text-dim">*__create_*</span>
                </div>
                <div className="mt-3 border-t border-border pt-3 text-text-dim">
                  auth: <span className="text-cyan">github-personal</span>{" "}
                  (oauth2, auto-injected)
                </div>
              </div>
            </div>

            <div className="terminal">
              <div className="terminal-header">
                <span className="terminal-dot bg-red/80" />
                <span className="terminal-dot bg-amber/80" />
                <span className="terminal-dot bg-green/80" />
                <span className="ml-3 text-[11px] text-text-dim">
                  ~/projects/infra
                </span>
              </div>
              <div className="p-4 sm:p-5 text-[11px] sm:text-xs leading-6 sm:leading-7">
                <div>
                  <span className="text-text-muted">$ </span>
                  <span className="text-text">cd ~/projects/infra</span>
                </div>
                <div className="mt-3 text-text-dim text-[10px] uppercase tracking-wider mb-2">
                  workspace: <span className="text-cyan">infra</span> (different
                  directory, different rules)
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-green">allow</span>
                  <span className="text-text-dim">aws__*, docker__*</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-red">deny</span>
                  <span className="text-text-dim">
                    github__create_*, slack__*
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-amber">approve</span>
                  <span className="text-text-dim">
                    aws__delete_*, docker__rm_*
                  </span>
                </div>
                <div className="mt-3 border-t border-border pt-3 text-text-dim">
                  auth: <span className="text-cyan">aws-prod</span> (env,
                  encrypted with age)
                </div>
              </div>
            </div>
          </div>

          <p className="mt-6 text-center text-xs text-text-dim max-w-xl mx-auto">
            Route rules match by path glob specificity, then tool pattern
            specificity, then priority. Deny stops the chain. No match falls
            back to the workspace default policy.
          </p>
        </div>
      </section>

      {/* ── Three Config Planes ── */}
      <section className="py-20 sm:py-28 bg-bg-alt border-t border-border">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-14">
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">
              Four ways to{" "}
              <span className="text-cyan">configure</span>
            </h2>
            <p className="mt-4 text-text-muted text-sm max-w-xl mx-auto">
              Desktop app for quick setup. YAML for version control. Web UI for
              visual management. MCP tools for AI-native configuration.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-surface border border-border p-6">
              <div className="text-[10px] text-text-dim uppercase tracking-wider mb-3">
                desktop app
              </div>
              <div className="terminal text-[11px] leading-6 p-4">
                <span className="text-cyan">$</span> {config.name} setup<br />
                <span className="text-green">{"✓"}</span> daemon started<br />
                <span className="text-green">{"✓"}</span> config updated
              </div>
              <p className="mt-4 text-xs text-text-dim">
                Native app with tray icon, one-click setup. Auto-starts
                daemon and configures Claude Desktop.
              </p>
            </div>

            <div className="bg-surface border border-border p-6">
              <div className="text-[10px] text-text-dim uppercase tracking-wider mb-3">
                yaml config
              </div>
              <div className="terminal text-[11px] leading-6 p-4">
                <span className="text-cyan">workspaces</span>:<br />
                {"  "}- <span className="text-cyan">name</span>:{" "}
                <span className="text-green">frontend</span>
                <br />
                {"    "}
                <span className="text-cyan">root_path</span>:{" "}
                <span className="text-green">~/projects/app</span>
                <br />
                {"    "}
                <span className="text-cyan">default_policy</span>:{" "}
                <span className="text-green">deny</span>
              </div>
              <p className="mt-4 text-xs text-text-dim">
                Checked into git. Seeded on startup. YAML-sourced items
                auto-pruned when removed from file.
              </p>
            </div>

            <div className="bg-surface border border-border p-6">
              <div className="text-[10px] text-text-dim uppercase tracking-wider mb-3">
                web ui + rest api
              </div>
              <div className="terminal text-[11px] leading-6 p-4">
                <span className="text-cyan">$</span> curl -X POST \<br />
                {"  "}localhost:8080/api/v1/routes \<br />
                {"  "}-d{" "}
                <span className="text-amber">
                  {
                    '\'{"name":"allow-github",...}\''
                  }
                </span>
              </div>
              <p className="mt-4 text-xs text-text-dim">
                Full CRUD on all entities. Real-time dashboard with SSE.
                API-created items persist across YAML reloads.
              </p>
            </div>

            <div className="bg-surface border border-border p-6">
              <div className="text-[10px] text-text-dim uppercase tracking-wider mb-3">
                mcp control server
              </div>
              <div className="terminal text-[11px] leading-6 p-4">
                <span className="text-cyan">$</span> {config.name}{" "}
                control-server
                <br />
                <span className="text-text-dim"># 15 MCP tools exposed</span>
                <br />
                <span className="text-text-dim">
                  # list_servers, create_route...
                </span>
              </div>
              <p className="mt-4 text-xs text-text-dim">
                Configure {config.name} from Claude or any MCP client. Read-only
                mode available for safe introspection.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* ── How It Works ── */}
      <section className="py-20 sm:py-28 border-t border-border">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-14">
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">
              Up and running in{" "}
              <span className="text-cyan">three steps</span>
            </h2>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 md:gap-0">
            {steps.map(({ num, title, description }, i) => (
              <div
                key={num}
                className="relative flex flex-col items-center text-center px-6"
              >
                {i < steps.length - 1 && (
                  <div className="hidden md:block absolute top-7 left-[calc(50%+2rem)] w-[calc(100%-4rem)] border-t border-dashed border-border" />
                )}
                <span className="text-3xl sm:text-4xl font-bold text-cyan/30 mb-4">
                  {num}
                </span>
                <h3 className="font-semibold text-base mb-2">{title}</h3>
                <p className="text-xs text-text-muted leading-relaxed max-w-xs">
                  {description}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── More Features ── */}
      <section className="py-20 sm:py-28 border-t border-border">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-14">
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">
              And <span className="text-cyan">everything else</span>
            </h2>
          </div>

          <div className="stagger grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {moreFeatures.map(({ icon: Icon, title, description }) => (
              <div
                key={title}
                className="flex items-start gap-4 bg-surface border border-border p-5 hover:border-border-hover transition-colors"
              >
                <div className="flex h-8 w-8 shrink-0 items-center justify-center bg-cyan/10 text-cyan border border-cyan/20">
                  <Icon className="w-3.5 h-3.5" />
                </div>
                <div>
                  <h3 className="font-semibold text-sm mb-1">{title}</h3>
                  <p className="text-xs leading-relaxed text-text-muted">
                    {description}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── CTA ── */}
      <section className="py-20 sm:py-28 border-t border-border">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="bg-surface-elevated border border-border p-10 sm:p-16 glow-cyan-sm text-center">
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight mb-4">
              Ready to take control?
            </h2>
            <p className="text-text-muted text-sm mb-10 max-w-lg mx-auto">
              Install {config.name} in seconds. Open source, local-first.
            </p>

            <div className="inline-flex items-center gap-3 bg-bg-alt border border-border px-5 py-3 mb-10">
              <Copy className="w-3.5 h-3.5 text-text-dim shrink-0" />
              <code className="text-xs sm:text-sm text-cyan">
                {config.installCmd}
              </code>
            </div>

            <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
              <Link
                href={config.github}
                target="_blank"
                rel="noopener noreferrer"
                className="group flex items-center gap-2 bg-cyan px-6 py-3 text-sm font-medium text-bg hover:bg-cyan-light transition-colors"
              >
                View on GitHub
                <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-0.5" />
              </Link>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
