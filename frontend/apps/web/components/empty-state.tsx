"use client"

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

/**
 * An empty state has one job: say what is not here, why, and what to do
 * next. The recovery action is the important part — an empty state without
 * one is a dead end.
 *
 * Only the error variant is coloured. "Nothing here yet" is not a problem
 * and should not be dressed as one; tinting a routine empty list red or
 * amber trains users to ignore the colour when it actually matters.
 */
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
        return <FileSearch className="size-6 text-fg-subtle" aria-hidden />
      case "shield":
        return <ShieldAlert className="size-6 text-warn" aria-hidden />
      case "alert":
        return <AlertCircle className="size-6 text-risk" aria-hidden />
      case "sparkles":
        return <Sparkles className="size-6 text-accent" aria-hidden />
      case "inbox":
      default:
        return <Inbox className="size-6 text-fg-subtle" aria-hidden />
    }
  }

  const isError = icon === "alert"

  return (
    <div
      role={isError ? "alert" : undefined}
      className={`mx-auto my-10 flex max-w-md flex-col items-center rounded-lg border border-dashed border-line p-10 text-center ${className}`}
    >
      <div className="flex size-12 items-center justify-center rounded-lg border border-line-subtle bg-raised">
        {renderIcon()}
      </div>
      <h3 className="mt-5 text-headline-sm text-fg">{title}</h3>
      <p className="mt-2 max-w-[46ch] text-body-md text-fg-muted">{description}</p>

      {(primaryAction || secondaryAction) && (
        <div className="mt-6 flex flex-wrap items-center justify-center gap-2.5">
          {primaryAction && (
            <Button variant="default" onClick={primaryAction.onClick}>
              {primaryAction.icon}
              {primaryAction.label}
            </Button>
          )}
          {secondaryAction && (
            <Button variant="ghost" onClick={secondaryAction.onClick}>
              {secondaryAction.label}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
