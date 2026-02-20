import { Sidebar } from "@/components/docs/sidebar";
import { MobileSidebar } from "@/components/docs/mobile-sidebar";

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen pt-14">
      <div className="mx-auto max-w-7xl flex">
        {/* Desktop sidebar */}
        <aside className="hidden lg:block w-64 shrink-0">
          <div className="sticky top-14 h-[calc(100vh-3.5rem)] overflow-y-auto border-r border-border py-6 px-4">
            <Sidebar />
          </div>
        </aside>

        {/* Mobile sidebar */}
        <MobileSidebar />

        {/* Main content */}
        <div className="flex-1 min-w-0">
          <div className="docs-content max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-8 pb-16">
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}
