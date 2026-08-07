"use client"

import * as React from "react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { Check, Copy } from "lucide-react"
import { cn } from "@workspace/ui/lib/utils"

interface MarkdownRendererProps {
  content: string
  className?: string
}

function CodeBlock({ children, className }: { children?: React.ReactNode; className?: string }) {
  const [copied, setCopied] = React.useState(false)
  const textContent = React.useMemo(() => {
    if (typeof children === "string") return children
    if (Array.isArray(children)) return children.join("")
    return String(children ?? "")
  }, [children])

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(textContent.trim())
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Fallback if clipboard API fails
    }
  }

  // Extract language if specified e.g. class "language-json"
  const match = /language-(\w+)/.exec(className || "")
  const lang = match ? match[1] : ""

  return (
    <div className="group relative my-3 overflow-hidden rounded-xl border border-line bg-[#0d1117] text-[#c9d1d9] shadow-sm">
      <div className="flex items-center justify-between border-b border-line/40 bg-[#161b22] px-3.5 py-1.5 text-[11px] font-mono text-[#8b949e]">
        <span>{lang ? lang.toUpperCase() : "CODE"}</span>
        <button
          type="button"
          onClick={handleCopy}
          className="flex items-center gap-1 rounded px-2 py-0.5 text-[11px] font-sans transition-colors hover:bg-white/10 hover:text-white"
        >
          {copied ? (
            <>
              <Check className="size-3 text-emerald-400" />
              <span className="text-emerald-400 font-medium">Copied</span>
            </>
          ) : (
            <>
              <Copy className="size-3" />
              <span>Copy</span>
            </>
          )}
        </button>
      </div>
      <pre className="overflow-x-auto p-3.5 font-mono text-xs leading-relaxed text-[#c9d1d9]">
        <code>{children}</code>
      </pre>
    </div>
  )
}

export function MarkdownRenderer({ content, className }: MarkdownRendererProps) {
  if (!content) return null

  return (
    <div className={cn("markdown-body text-xs leading-relaxed text-foreground/90 space-y-2.5", className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h1: ({ children }) => (
            <h1 className="text-xl font-bold font-display tracking-tight text-foreground border-b border-line/60 pb-1.5 mt-4 mb-2">
              {children}
            </h1>
          ),
          h2: ({ children }) => (
            <h2 className="text-lg font-bold font-display tracking-tight text-foreground border-b border-line/40 pb-1 mt-3.5 mb-2">
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3 className="text-base font-semibold font-display text-foreground mt-3 mb-1.5">
              {children}
            </h3>
          ),
          h4: ({ children }) => (
            <h4 className="text-sm font-semibold text-foreground mt-2 mb-1">
              {children}
            </h4>
          ),
          p: ({ children }) => (
            <p className="leading-relaxed my-1.5 text-foreground/90">{children}</p>
          ),
          strong: ({ children }) => (
            <strong className="font-semibold text-foreground">{children}</strong>
          ),
          em: ({ children }) => (
            <em className="italic text-foreground/90">{children}</em>
          ),
          ul: ({ children }) => (
            <ul className="list-disc list-outside pl-4 space-y-1 my-2 text-foreground/90">
              {children}
            </ul>
          ),
          ol: ({ children }) => (
            <ol className="list-decimal list-outside pl-4 space-y-1 my-2 text-foreground/90">
              {children}
            </ol>
          ),
          li: ({ children }) => (
            <li className="leading-relaxed">{children}</li>
          ),
          blockquote: ({ children }) => (
            <blockquote className="border-l-3 border-accent bg-accent/5 dark:bg-accent/10 px-3.5 py-2 rounded-r-lg my-2.5 text-xs text-foreground/90 italic shadow-2xs">
              {children}
            </blockquote>
          ),
          table: ({ children }) => (
            <div className="my-3 overflow-x-auto rounded-lg border border-line shadow-2xs">
              <table className="w-full border-collapse text-left text-xs">
                {children}
              </table>
            </div>
          ),
          thead: ({ children }) => (
            <thead className="bg-cream-200/60 dark:bg-surface border-b border-line font-semibold text-foreground">
              {children}
            </thead>
          ),
          tbody: ({ children }) => (
            <tbody className="divide-y divide-line/60 bg-background">{children}</tbody>
          ),
          tr: ({ children }) => (
            <tr className="transition-colors hover:bg-cream-100/40 dark:hover:bg-surface/50">
              {children}
            </tr>
          ),
          th: ({ children }) => (
            <th className="px-3 py-2 font-bold text-foreground bg-surface/40">{children}</th>
          ),
          td: ({ children }) => (
            <td className="px-3 py-2 text-foreground/90 align-top">{children}</td>
          ),
          hr: () => <hr className="my-3.5 border-line" />,
          a: ({ href, children }) => (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="text-accent hover:underline font-medium"
            >
              {children}
            </a>
          ),
          code: ({ node, className, children, ...props }: any) => {
            const isInline = !node || node.position?.start?.line === node.position?.end?.line && !className?.includes("language-")
            if (isInline) {
              return (
                <code
                  className="rounded bg-cream-200/80 dark:bg-raised/80 px-1.5 py-0.5 font-mono text-[0.85em] font-medium text-foreground border border-line-subtle"
                  {...props}
                >
                  {children}
                </code>
              )
            }
            return <CodeBlock className={className}>{children}</CodeBlock>
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}
