import type { MDXComponents } from "mdx/types";
import Link from "next/link";
import { CodeBlock } from "@/components/docs/code-block";

export function useMDXComponents(components: MDXComponents): MDXComponents {
  return {
    h1: ({ children, id }) => (
      <h1 id={id} className="text-2xl font-bold text-text mt-0 mb-6">
        {children}
      </h1>
    ),
    h2: ({ children, id }) => (
      <h2
        id={id}
        className="text-lg font-bold text-text mt-10 mb-4 pt-6 border-t border-border scroll-mt-20"
      >
        {children}
      </h2>
    ),
    h3: ({ children, id }) => (
      <h3
        id={id}
        className="text-base font-semibold text-text mt-8 mb-3 scroll-mt-20"
      >
        {children}
      </h3>
    ),
    p: ({ children }) => (
      <p className="text-sm text-text-muted leading-relaxed mb-4">
        {children}
      </p>
    ),
    a: ({ href, children }) => {
      const isExternal = href?.startsWith("http");
      if (isExternal) {
        return (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-cyan hover:text-cyan-light underline underline-offset-2 transition-colors"
          >
            {children}
          </a>
        );
      }
      return (
        <Link
          href={href || ""}
          className="text-cyan hover:text-cyan-light underline underline-offset-2 transition-colors"
        >
          {children}
        </Link>
      );
    },
    ul: ({ children }) => (
      <ul className="text-sm text-text-muted leading-relaxed mb-4 ml-4 list-disc marker:text-text-dim space-y-1">
        {children}
      </ul>
    ),
    ol: ({ children }) => (
      <ol className="text-sm text-text-muted leading-relaxed mb-4 ml-4 list-decimal marker:text-text-dim space-y-1">
        {children}
      </ol>
    ),
    li: ({ children }) => <li className="pl-1">{children}</li>,
    code: ({ children, className }) => {
      // Fenced code blocks get a className like "language-ts"
      if (className) {
        const lang = className.replace("language-", "");
        return (
          <CodeBlock language={lang}>
            {String(children).replace(/\n$/, "")}
          </CodeBlock>
        );
      }
      // Inline code
      return (
        <code className="px-1.5 py-0.5 bg-surface text-cyan text-[0.85em] border border-border rounded">
          {children}
        </code>
      );
    },
    pre: ({ children }) => {
      // The <pre> wraps a <code> — just pass through since CodeBlock handles styling
      return <>{children}</>;
    },
    table: ({ children }) => (
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm border border-border">{children}</table>
      </div>
    ),
    thead: ({ children }) => (
      <thead className="bg-surface text-text-muted text-xs uppercase tracking-wider">
        {children}
      </thead>
    ),
    th: ({ children }) => (
      <th className="text-left px-3 py-2 border-b border-border font-medium">
        {children}
      </th>
    ),
    td: ({ children }) => (
      <td className="px-3 py-2 border-b border-border text-text-muted">
        {children}
      </td>
    ),
    tr: ({ children }) => <tr className="hover:bg-surface/50">{children}</tr>,
    blockquote: ({ children }) => (
      <blockquote className="border-l-2 border-cyan/40 pl-4 my-4 text-text-dim italic">
        {children}
      </blockquote>
    ),
    hr: () => <hr className="my-8 border-border" />,
    strong: ({ children }) => (
      <strong className="text-text font-semibold">{children}</strong>
    ),
    ...components,
  };
}
