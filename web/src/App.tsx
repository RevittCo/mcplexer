import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { TooltipProvider } from '@/components/ui/tooltip'
import { AppLayout } from '@/components/layout/AppLayout'
import { DashboardPage } from '@/pages/DashboardPage'
import { AuditPage } from '@/pages/AuditPage'
import { WorkspacesPage } from '@/pages/config/WorkspacesPage'
import { DownstreamsPage } from '@/pages/config/DownstreamsPage'
import { RoutesPage } from '@/pages/config/RoutesPage'
import { AuthScopesPage } from '@/pages/config/AuthScopesPage'
import { OAuthProvidersPage } from '@/pages/config/OAuthProvidersPage'
import { DryRunPage } from '@/pages/DryRunPage'

function App() {
  return (
    <TooltipProvider>
    <BrowserRouter>
      <AppLayout>
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/config/workspaces" element={<WorkspacesPage />} />
          <Route path="/config/downstreams" element={<DownstreamsPage />} />
          <Route path="/config/routes" element={<RoutesPage />} />
          <Route path="/config/auth-scopes" element={<AuthScopesPage />} />
          <Route path="/config/oauth-providers" element={<OAuthProvidersPage />} />
          <Route path="/dry-run" element={<DryRunPage />} />
        </Routes>
      </AppLayout>
    </BrowserRouter>
    </TooltipProvider>
  )
}

export default App
