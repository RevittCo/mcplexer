"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { getPrevNext } from "@/lib/docs-nav";

export function PrevNextNav() {
  const pathname = usePathname();
  const cleanPath = pathname.replace(/^\/mcplexer/, "");
  const { prev, next } = getPrevNext(cleanPath);

  if (!prev && !next) return null;

  return (
    <div className="flex justify-between items-center mt-12 pt-6 border-t border-border">
      {prev ? (
        <Link
          href={prev.href}
          className="flex items-center gap-1.5 text-xs text-text-muted hover:text-cyan transition-colors"
        >
          <ChevronLeft className="w-3.5 h-3.5" />
          {prev.title}
        </Link>
      ) : (
        <div />
      )}
      {next ? (
        <Link
          href={next.href}
          className="flex items-center gap-1.5 text-xs text-text-muted hover:text-cyan transition-colors"
        >
          {next.title}
          <ChevronRight className="w-3.5 h-3.5" />
        </Link>
      ) : (
        <div />
      )}
    </div>
  );
}
