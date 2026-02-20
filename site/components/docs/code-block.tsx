"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";

interface CodeBlockProps {
  children: string;
  language?: string;
  filename?: string;
}

export function CodeBlock({ children, language, filename }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);

  function handleCopy() {
    navigator.clipboard.writeText(children);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="relative group mb-4 border border-border rounded overflow-hidden">
      {(filename || language) && (
        <div className="flex items-center justify-between px-3 py-1.5 bg-surface border-b border-border">
          <span className="text-[10px] text-text-dim font-medium">
            {filename || language}
          </span>
        </div>
      )}
      <div className="relative">
        <pre className="overflow-x-auto p-4 bg-bg-alt text-sm leading-relaxed">
          <code className="text-text-muted">{children}</code>
        </pre>
        <button
          onClick={handleCopy}
          className="absolute top-2 right-2 p-1.5 bg-surface border border-border rounded opacity-0 group-hover:opacity-100 transition-opacity text-text-dim hover:text-text"
          aria-label="Copy code"
        >
          {copied ? (
            <Check className="w-3.5 h-3.5 text-green" />
          ) : (
            <Copy className="w-3.5 h-3.5" />
          )}
        </button>
      </div>
    </div>
  );
}
