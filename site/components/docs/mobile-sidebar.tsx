"use client";

import { useState } from "react";
import { Menu, X } from "lucide-react";
import { Sidebar } from "./sidebar";

export function MobileSidebar() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="lg:hidden fixed bottom-4 right-4 z-50 p-3 bg-surface border border-border rounded-full shadow-lg text-text-muted hover:text-text transition-colors"
        aria-label="Open navigation"
      >
        <Menu className="w-5 h-5" />
      </button>

      {open && (
        <>
          <div
            className="lg:hidden fixed inset-0 z-50 bg-bg/80 backdrop-blur-sm"
            onClick={() => setOpen(false)}
          />
          <div className="lg:hidden fixed inset-y-0 left-0 z-50 w-72 bg-bg border-r border-border overflow-y-auto">
            <div className="flex items-center justify-between p-4 border-b border-border">
              <span className="text-xs font-medium text-text">Navigation</span>
              <button
                onClick={() => setOpen(false)}
                className="p-1 text-text-muted hover:text-text transition-colors"
                aria-label="Close navigation"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <div className="p-4" onClick={() => setOpen(false)}>
              <Sidebar />
            </div>
          </div>
        </>
      )}
    </>
  );
}
