"use client"

import * as React from "react"
import type { ReactNode } from "react"
import { AlertCircle, FileSearch, Inbox, ShieldAlert, Sparkles } from "lucide-react"
import { Button } from "@workspace/ui/components/button"

interface EmptyStateProps {
  icon?: "inbox" | "search" | "shield" | "alert" | "sparkles" | ReactNode
  title: string
  description: string
  primaryAction?: {
    label: string
    onClick: () => void
    icon?: ReactNode
  }
  secondaryAction?: {
    label: string
    onClick: () => void
  }
  className?: string
}

export function EmptyState({
  icon = "inbox",
  title,
  description,
  primaryAction,
  secondaryAction,
  className = "",
}: EmptyStateProps) {
  const renderIcon = () => {
    if (typeof icon !== "string") return icon
    switch (icon) {
      case "search":
        return <FileSearch className="size-8 text-text-dim" />
      case "shield":
        return <ShieldAlert className="size-8 text-warn" />
      case "alert":
        return <AlertCircle className="size-8 text-risk" />
      case "sparkles":
        return <Sparkles className="size-8 text-lavender" />
      case "inbox":
      default:
        return <Inbox className="size-8 text-text-dim" />
    }
  }

  return (
    <div
      className={`flex flex-col items-center justify-center text-center p-8 md:p-12 rounded-2xl border border-dashed border-line bg-surface/60 max-w-lg mx-auto my-8 ${className}`}
    >
      <div className="flex size-16 items-center justify-center rounded-2xl bg-cream-200/80 shadow-inner mb-4">
        {renderIcon()}
      </div>
      <h3 className="font-display text-xl text-foreground font-semibold tracking-tight">
        {title}
      </h3>
      <p className="mt-2 text-sm text-text-dim max-w-sm leading-relaxed">
        {description}
      </p>

      {(primaryAction || secondaryAction) && (
        <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
          {primaryAction && (
            <Button variant="default" onClick={primaryAction.onClick}>
              {primaryAction.icon}
              {primaryAction.label}
            </Button>
          )}
          {secondaryAction && (
            <Button variant="outline" onClick={secondaryAction.onClick}>
              {secondaryAction.label}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
