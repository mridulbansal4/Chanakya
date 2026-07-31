"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { AnimatePresence, motion } from "framer-motion"
import {
  ArrowRight,
  BookOpen,
  CheckCircle2,
  FileCheck,
  FileText,
  Home,
  Network,
  Radio,
  Search,
  ShieldCheck,
  Zap,
} from "lucide-react"

interface NavItem {
  id: string
  title: string
  subtitle: string
  href: string
  icon: React.ReactNode
  badge: string
}

const ITEMS: NavItem[] = [
  {
    id: "overview",
    title: "Executive Overview",
    subtitle: "Compliance posture & live DAG graph",
    href: "/",
    icon: <Home className="size-4 text-emerald-400" />,
    badge: "Dashboard",
  },
  {
    id: "reg-feed",
    title: "Regulatory Feed",
    subtitle: "Live SEBI circular detection & MITC workflow",
    href: "/regulatory-feed",
    icon: <Radio className="size-4 text-cyan-400" />,
    badge: "Automation",
  },
  {
    id: "register",
    title: "Obligation Register",
    subtitle: "Full extracted regulatory obligation matrix",
    href: "/register",
    icon: <BookOpen className="size-4 text-purple-400" />,
    badge: "Matrix",
  },
  {
    id: "blast",
    title: "Blast Radius Simulator",
    subtitle: "Preview ripple effects of clause amendments",
    href: "/amendments",
    icon: <Zap className="size-4 text-amber-400" />,
    badge: "Simulation",
  },
  {
    id: "evidence",
    title: "Evidence Coverage & Gaps",
    subtitle: "Read-only system connectors & remediation tickets",
    href: "/evidence",
    icon: <FileCheck className="size-4 text-emerald-400" />,
    badge: "Audit",
  },
  {
    id: "review",
    title: "Review Queue Inbox",
    subtitle: "Obligations awaiting compliance officer sign-off",
    href: "/review",
    icon: <ShieldCheck className="size-4 text-amber-400" />,
    badge: "Inbox",
  },
  {
    id: "policy",
    title: "Automated Policy Engine",
    subtitle: "OPA / Rego deterministic code checks",
    href: "/policy",
    icon: <CheckCircle2 className="size-4 text-cyan-400" />,
    badge: "OPA Code",
  },
  {
    id: "audit",
    title: "Compliance Lineage Audit",
    subtitle: "Time-travel audit trail & provenance graph",
    href: "/audit",
    icon: <Network className="size-4 text-purple-400" />,
    badge: "Provenance",
  },
  {
    id: "feed",
    title: "Machine-Readable Feed",
    subtitle: "SupTech feed for regulatory system ingestion",
    href: "/feed",
    icon: <FileText className="size-4 text-emerald-400" />,
    badge: "SupTech",
  },
]

export function CommandPalette() {
  const [open, setOpen] = React.useState(false)
  const [query, setQuery] = React.useState("")
  const [selectedIndex, setSelectedIndex] = React.useState(0)
  const router = useRouter()

  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault()
        setOpen((prev) => !prev)
      } else if (e.key === "Escape") {
        setOpen(false)
      }
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [])

  const filtered = React.useMemo(() => {
    if (!query.trim()) return ITEMS
    const q = query.toLowerCase()
    return ITEMS.filter(
      (item) =>
        item.title.toLowerCase().includes(q) ||
        item.subtitle.toLowerCase().includes(q) ||
        item.badge.toLowerCase().includes(q)
    )
  }, [query])

  React.useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  const selectItem = (item: NavItem) => {
    setOpen(false)
    setQuery("")
    router.push(item.href)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault()
      setSelectedIndex((i) => (i + 1) % Math.max(1, filtered.length))
    } else if (e.key === "ArrowUp") {
      e.preventDefault()
      setSelectedIndex((i) => (i - 1 + filtered.length) % Math.max(1, filtered.length))
    } else if (e.key === "Enter" && filtered[selectedIndex]) {
      e.preventDefault()
      selectItem(filtered[selectedIndex]!)
    }
  }

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="hidden md:inline-flex items-center gap-2.5 rounded-full border border-white/10 bg-white/5 px-3.5 py-1.5 text-xs text-on-ink-dim hover:text-on-ink hover:bg-white/10 hover:border-white/20 transition-all shadow-inner group"
        title="Search operating system (Cmd+K)"
      >
        <Search className="size-3.5 text-lavender transition-transform group-hover:scale-110" />
        <span className="font-medium">Search OS…</span>
        <kbd className="tnum rounded-md bg-white/10 px-1.5 py-0.5 text-[10px] font-mono text-on-ink-dim border border-white/10">
          ⌘K
        </kbd>
      </button>

      <AnimatePresence>
        {open && (
          <div className="fixed inset-0 z-50 flex items-start justify-center pt-24 px-4 bg-black/75 backdrop-blur-xl">
            <motion.div
              initial={{ opacity: 0, scale: 0.94, y: -12 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.94, y: -12 }}
              transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
              className="w-full max-w-xl overflow-hidden rounded-3xl border border-white/15 bg-[#141417]/95 shadow-[0_25px_60px_-15px_rgba(0,0,0,0.7)] text-white"
            >
              {/* Clean search bar input */}
              <div className="flex items-center border-b border-white/10 px-5 py-4 bg-white/5">
                <Search className="size-5 text-lavender shrink-0 mr-3" />
                <input
                  autoFocus
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Type a command or search destination…"
                  className="w-full bg-transparent text-white placeholder:text-white/40 outline-none text-sm font-medium tracking-wide"
                />
                <kbd className="tnum rounded-lg bg-white/10 px-2 py-1 text-xs text-white/50 font-mono shrink-0 border border-white/10">
                  ESC
                </kbd>
              </div>

              {/* Items List */}
              <div className="max-h-84 overflow-y-auto p-3 space-y-1">
                {filtered.length === 0 ? (
                  <div className="p-10 text-center text-xs text-white/40 font-medium">
                    No operating system commands match &quot;{query}&quot;
                  </div>
                ) : (
                  <>
                    <div className="px-3 py-1.5 eyebrow text-white/40 text-[10px]">
                      Navigation Commands ({filtered.length})
                    </div>
                    {filtered.map((item, idx) => {
                      const isSelected = idx === selectedIndex
                      return (
                        <button
                          key={item.id}
                          onClick={() => selectItem(item)}
                          onMouseEnter={() => setSelectedIndex(idx)}
                          className={`w-full flex items-center justify-between px-4 py-3 rounded-2xl text-left transition-all ${
                            isSelected
                              ? "bg-gradient-to-r from-blue-900/60 to-slate-900/60 border border-blue-500/40 text-white shadow-lg translate-x-1"
                              : "hover:bg-white/5 text-white/80 border border-transparent"
                          }`}
                        >
                          <div className="flex items-center gap-3.5 min-w-0">
                            <div
                              className={`p-2.5 rounded-xl transition-transform ${
                                isSelected ? "bg-white/15 scale-105" : "bg-white/5"
                              }`}
                            >
                              {item.icon}
                            </div>
                            <div className="min-w-0 space-y-0.5">
                              <div className="text-sm font-semibold truncate tracking-tight">
                                {item.title}
                              </div>
                              <div className="text-xs text-white/50 truncate">
                                {item.subtitle}
                              </div>
                            </div>
                          </div>

                          <div className="flex items-center gap-2 shrink-0 ml-2">
                            <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-white/10 text-white/60 border border-white/5">
                              {item.badge}
                            </span>
                            <ArrowRight
                              className={`size-4 transition-transform ${
                                isSelected ? "translate-x-1 text-lavender opacity-100" : "opacity-0"
                              }`}
                            />
                          </div>
                        </button>
                      )
                    })}
                  </>
                )}
              </div>

              {/* Footer */}
              <div className="flex items-center justify-between border-t border-white/10 bg-black/40 px-5 py-2.5 text-xs text-white/40 font-mono">
                <div className="flex items-center gap-3 text-[11px]">
                  <span>↑↓ Navigate</span>
                  <span>•</span>
                  <span>↵ Select</span>
                </div>
                <div className="text-[10px] text-lavender font-bold">CHANAKYA Enterprise OS</div>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </>
  )
}
