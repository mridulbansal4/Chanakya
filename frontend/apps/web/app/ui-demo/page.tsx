"use client"

import * as React from "react"
import { AvatarStack } from "@/components/kibo-ui/avatar-stack"
import { Sparkles, Layers, Zap, Command, CheckCircle2 } from "lucide-react"

export default function UIDemoPage() {
  return (
    <div className="min-h-screen bg-background p-8 text-foreground space-y-10 max-w-5xl mx-auto">
      {/* Header */}
      <div className="space-y-3 border-b border-border pb-6">
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 text-primary text-sm font-medium">
          <Sparkles className="size-4" />
          <span>UI Libraries Configured</span>
        </div>
        <h1 className="text-4xl font-semibold tracking-tight">
          Kibo UI & Forge UI Integration
        </h1>
        <p className="text-muted-foreground text-lg">
          Your project is now set up with Kibo UI and Forge UI components.
        </p>
      </div>

      {/* Kibo UI Live Demo */}
      <section className="space-y-4 border border-border rounded-xl p-6 bg-card/50 shadow-elev-1">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Layers className="size-5 text-primary" />
            <h2 className="text-xl font-bold">Kibo UI Demo — Avatar Stack</h2>
          </div>
          <span className="text-xs px-2.5 py-1 rounded bg-muted font-mono text-muted-foreground">
            @/components/kibo-ui/avatar-stack
          </span>
        </div>

        <p className="text-sm text-muted-foreground">
          Interactive animated stack from Kibo UI (<code className="text-xs">https://www.kibo-ui.com/</code>). Hover over to animate!
        </p>

        <div className="p-6 bg-background rounded-lg border border-border flex items-center gap-6">
          <AvatarStack animate size={44}>
            <img
              src="https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=100&auto=format&fit=crop&q=80"
              alt="User 1"
              className="object-cover"
            />
            <img
              src="https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=100&auto=format&fit=crop&q=80"
              alt="User 2"
              className="object-cover"
            />
            <img
              src="https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=100&auto=format&fit=crop&q=80"
              alt="User 3"
              className="object-cover"
            />
            <img
              src="https://images.unsplash.com/photo-1500648767791-00dcc994a43e?w=100&auto=format&fit=crop&q=80"
              alt="User 4"
              className="object-cover"
            />
          </AvatarStack>

          <div className="text-sm">
            <span className="font-semibold text-foreground block">Active Team Members</span>
            <span className="text-xs text-muted-foreground">4 members online now</span>
          </div>
        </div>
      </section>

      {/* CLI Installation Cheat Sheet */}
      <section className="space-y-4 border border-border rounded-xl p-6 bg-card/50 shadow-elev-1">
        <div className="flex items-center gap-2">
          <Zap className="size-5 text-warn" />
          <h2 className="text-xl font-bold">Quick CLI Commands</h2>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          {/* Kibo UI Commands */}
          <div className="space-y-2 p-4 rounded-lg bg-background border border-border">
            <h3 className="font-semibold flex items-center gap-2">
              <CheckCircle2 className="size-4 text-ok" />
              Add Kibo UI Components
            </h3>
            <p className="text-xs text-muted-foreground">
              Run inside <code className="text-xs font-mono">frontend/apps/web</code>:
            </p>
            <div className="p-3 bg-muted rounded font-mono text-xs overflow-x-auto space-y-1">
              <p className="text-ok"># Add any Kibo UI component</p>
              <p>npx kibo-ui add gantt</p>
              <p>npx kibo-ui add kanban</p>
              <p>npx kibo-ui add dropzone</p>
              <p>npx kibo-ui add code-block</p>
            </div>
          </div>

          {/* Forge UI Commands */}
          <div className="space-y-2 p-4 rounded-lg bg-background border border-border">
            <h3 className="font-semibold flex items-center gap-2">
              <CheckCircle2 className="size-4 text-ok" />
              Add Forge UI Components
            </h3>
            <p className="text-xs text-muted-foreground">
              Run inside <code className="text-xs font-mono">frontend/apps/web</code>:
            </p>
            <div className="p-3 bg-muted rounded font-mono text-xs overflow-x-auto space-y-1">
              <p className="text-accent"># Add Forge UI components via shadcn CLI</p>
              <p>npx shadcn@latest add &quot;https://forgeui.in/r/animated-form.json&quot;</p>
              <p>npx shadcn@latest add &quot;https://forgeui.in/r/animated-tabs.json&quot;</p>
              <p>npx shadcn@latest add &quot;https://forgeui.in/r/bot-detection.json&quot;</p>
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}
