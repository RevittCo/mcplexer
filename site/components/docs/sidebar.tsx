"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { docsNav } from "@/lib/docs-nav";

export function Sidebar() {
  const pathname = usePathname();
  // Strip basePath for matching
  const cleanPath = pathname.replace(/^\/mcplexer/, "");

  return (
    <nav className="space-y-6">
      {docsNav.map((section) => (
        <div key={section.title}>
          <h4 className="text-[10px] font-medium uppercase tracking-wider text-text-dim mb-2">
            {section.title}
          </h4>
          <ul className="space-y-0.5">
            {section.items.map((item) => {
              const isActive = cleanPath === item.href;
              return (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    className={`block px-2 py-1.5 text-xs transition-colors rounded ${
                      isActive
                        ? "text-cyan bg-cyan/10 border-l-2 border-cyan"
                        : "text-text-muted hover:text-text hover:bg-surface-hover"
                    }`}
                  >
                    {item.title}
                  </Link>
                </li>
              );
            })}
          </ul>
        </div>
      ))}
    </nav>
  );
}
