import { Link, useLocation } from 'react-router-dom'
import {
  ChevronDown,
  FileText,
  GitBranch,
  Globe,
  KeyRound,
  LayoutDashboard,
  Lock,
  Menu,
  Play,
  Server,
  ShieldCheck,
  Zap,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useState } from 'react'
import { Sheet, SheetContent } from '@/components/ui/sheet'

interface NavItem {
  label: string
  href: string
  icon: React.ReactNode
}

const mainNav: NavItem[] = [
  { label: 'Dashboard', href: '/', icon: <LayoutDashboard className="h-4 w-4" /> },
  { label: 'Quick Setup', href: '/setup', icon: <Zap className="h-4 w-4" /> },
  { label: 'Approvals', href: '/approvals', icon: <ShieldCheck className="h-4 w-4" /> },
  { label: 'Audit Logs', href: '/audit', icon: <FileText className="h-4 w-4" /> },
]

const configNav: NavItem[] = [
  { label: 'Workspaces', href: '/config/workspaces', icon: <Globe className="h-4 w-4" /> },
  { label: 'Route Rules', href: '/config/routes', icon: <GitBranch className="h-4 w-4" /> },
  { label: 'Credentials', href: '/config/auth-scopes', icon: <Lock className="h-4 w-4" /> },
  { label: 'Servers', href: '/config/downstreams', icon: <Server className="h-4 w-4" /> },
]

const advancedNav: NavItem[] = [
  { label: 'OAuth Providers', href: '/config/oauth-providers', icon: <KeyRound className="h-4 w-4" /> },
]

const toolsNav: NavItem[] = [
  { label: 'Dry Run', href: '/dry-run', icon: <Play className="h-4 w-4" /> },
]

function NavLink({ item, onNavigate }: { item: NavItem; onNavigate?: () => void }) {
  const location = useLocation()
  const active = location.pathname === item.href

  return (
    <Link
      to={item.href}
      onClick={onNavigate}
      className={cn(
        'group relative flex items-center gap-3 rounded-r-md px-3 py-2 text-[13px] tracking-wide transition-all duration-150',
        active
          ? 'border-l-[2.5px] border-primary bg-sidebar-accent text-sidebar-accent-foreground'
          : 'border-l-[2.5px] border-transparent text-sidebar-foreground hover:translate-x-0.5 hover:text-foreground',
      )}
    >
      <span
        className={cn(
          'shrink-0 transition-colors duration-150',
          active ? 'text-primary' : 'text-sidebar-foreground group-hover:text-foreground',
        )}
      >
        {item.icon}
      </span>
      <span className="truncate">{item.label}</span>
    </Link>
  )
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="px-4 pb-1.5 pt-1 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/60">
      {children}
    </div>
  )
}

function McplexerLogo({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 32 32"
      fill="none"
      className={className}
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M3 7H9L17 16" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M3 16H17" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
      <path d="M3 25H9L17 16" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M17 16H29" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
      <circle cx="17" cy="16" r="2.5" fill="currentColor" />
    </svg>
  )
}

function BrandHeader() {
  return (
    <div className="flex h-14 items-center gap-2.5 border-b border-sidebar-border px-4">
      <div className="relative flex items-center justify-center">
        <McplexerLogo className="relative z-10 h-5 w-5 text-primary" />
        <div className="absolute inset-0 rounded-full bg-primary/20 blur-md" />
      </div>
      <span className="text-[15px] font-semibold tracking-tight text-foreground">
        MCPlexer
      </span>
      <span className="ml-auto bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
        alpha
      </span>
    </div>
  )
}

function StatusBar() {
  const displayHost = window.location.host || 'localhost'
  return (
    <div className="border-t border-sidebar-border px-4 py-3">
      <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
        <span className="relative flex h-2 w-2">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
          <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
        </span>
        <span>Connected</span>
        <span className="ml-auto text-muted-foreground/50">{displayHost}</span>
      </div>
    </div>
  )
}

function SidebarNav({ configOpen, setConfigOpen, onNavigate }: {
  configOpen: boolean
  setConfigOpen: (v: boolean) => void
  onNavigate?: () => void
}) {
  return (
    <nav className="flex-1 space-y-0.5 py-3 pr-2">
      <SectionLabel>Overview</SectionLabel>
      {mainNav.map((item) => (
        <NavLink key={item.href} item={item} onNavigate={onNavigate} />
      ))}

      <div className="pt-3">
        <button
          onClick={() => setConfigOpen(!configOpen)}
          className="flex w-full items-center gap-2 px-4 pb-1.5 pt-1 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/60 transition-colors hover:text-muted-foreground"
        >
          Configuration
          <ChevronDown
            className={cn(
              'ml-auto h-3 w-3 transition-transform duration-200',
              configOpen && 'rotate-180',
            )}
          />
        </button>
      </div>

      <div
        className={cn(
          'grid transition-[grid-template-rows] duration-200 ease-out',
          configOpen ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]',
        )}
      >
        <div className="overflow-hidden">
          <div className="space-y-0.5">
            {configNav.map((item) => (
              <NavLink key={item.href} item={item} onNavigate={onNavigate} />
            ))}
            <div className="border-t border-sidebar-border/40 mt-1.5 pt-1.5">
              {advancedNav.map((item) => (
                <NavLink key={item.href} item={item} onNavigate={onNavigate} />
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="pt-3">
        <SectionLabel>Tools</SectionLabel>
        {toolsNav.map((item) => (
          <NavLink key={item.href} item={item} onNavigate={onNavigate} />
        ))}
      </div>
    </nav>
  )
}

export function AppLayout({ children }: { children: React.ReactNode }) {
  const [configOpen, setConfigOpen] = useState(true)
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <div className="flex min-h-screen">
      {/* Desktop sidebar */}
      <aside className="hidden w-56 flex-col border-r border-sidebar-border bg-sidebar-background md:flex">
        <BrandHeader />
        <SidebarNav configOpen={configOpen} setConfigOpen={setConfigOpen} />
        <StatusBar />
      </aside>

      {/* Mobile sheet */}
      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent side="left" className="w-56 bg-sidebar-background p-0" showCloseButton={false} aria-describedby={undefined}>
          <span className="sr-only">Navigation</span>
          <BrandHeader />
          <SidebarNav
            configOpen={configOpen}
            setConfigOpen={setConfigOpen}
            onNavigate={() => setMobileOpen(false)}
          />
          <StatusBar />
        </SheetContent>
      </Sheet>

      <div className="flex min-w-0 flex-1 flex-col bg-background">
        {/* Mobile header */}
        <div className="flex h-14 items-center gap-2.5 border-b border-border px-4 md:hidden">
          <button
            onClick={() => setMobileOpen(true)}
            className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <Menu className="h-5 w-5" />
          </button>
          <McplexerLogo className="h-4 w-4 text-primary" />
          <span className="text-sm font-semibold tracking-tight">MCPlexer</span>
        </div>

        <div className="h-px bg-gradient-to-r from-primary/20 via-primary/5 to-transparent" />
        <main className="flex-1 overflow-x-hidden p-4 md:p-6">{children}</main>
      </div>
    </div>
  )
}
