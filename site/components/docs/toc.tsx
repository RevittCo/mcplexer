"use client";

import { useEffect, useState } from "react";

interface TocItem {
  id: string;
  text: string;
  level: number;
}

export function TableOfContents() {
  const [headings, setHeadings] = useState<TocItem[]>([]);
  const [activeId, setActiveId] = useState("");

  useEffect(() => {
    const content = document.querySelector(".docs-content");
    if (!content) return;

    const elements = content.querySelectorAll("h2[id], h3[id]");
    const items: TocItem[] = Array.from(elements).map((el) => ({
      id: el.id,
      text: el.textContent || "",
      level: el.tagName === "H2" ? 2 : 3,
    }));
    setHeadings(items);

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveId(entry.target.id);
          }
        }
      },
      { rootMargin: "-80px 0px -80% 0px", threshold: 0 }
    );

    elements.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, []);

  if (headings.length === 0) return null;

  return (
    <nav className="space-y-1">
      <h4 className="text-[10px] font-medium uppercase tracking-wider text-text-dim mb-3">
        On this page
      </h4>
      {headings.map((h) => (
        <a
          key={h.id}
          href={`#${h.id}`}
          className={`block text-xs py-0.5 transition-colors ${
            h.level === 3 ? "pl-3" : ""
          } ${
            activeId === h.id
              ? "text-cyan"
              : "text-text-dim hover:text-text-muted"
          }`}
        >
          {h.text}
        </a>
      ))}
    </nav>
  );
}
