"use client"

import * as React from "react"
import { motion, useReducedMotion } from "framer-motion"

import { StageRail } from "@/components/amendment/kit"
import {
  AuditPack,
  BlastRadius,
  ClauseDiff,
  Evidence,
  Execution,
  GraphUpdate,
  HumanReview,
  Inbox_,
  Obligations,
  Processing,
  Workflows,
} from "@/components/amendment/steps"

/**
 * Regulatory Amendment Simulation — a self-contained, guided demo of the full
 * lifecycle of one real SEBI circular (MITC, 17 Feb 2025). It is an extension:
 * it adds a route and a nav item, and touches nothing else. All data is scripted
 * in lib/amendment-sim.ts; no backend calls, no existing pages changed.
 */
export default function RegulatoryFeedPage() {
  const [step, setStep] = React.useState(0)
  const reduce = useReducedMotion()
  const scrollRef = React.useRef<HTMLDivElement>(null)

  const go = (n: number) => setStep(n)
  React.useEffect(() => {
    scrollRef.current?.scrollTo({ top: 0 })
  }, [step])

  const screen = () => {
    switch (step) {
      case 0:
        return <Inbox_ onProcess={() => go(1)} />
      case 1:
        return <Processing onNext={() => go(2)} />
      case 2:
        return <ClauseDiff onNext={() => go(3)} />
      case 3:
        return <Obligations onNext={() => go(4)} />
      case 4:
        return <GraphUpdate onNext={() => go(5)} />
      case 5:
        return <BlastRadius onNext={() => go(6)} />
      case 6:
        return <Workflows onNext={() => go(7)} />
      case 7:
        return <HumanReview onApprove={() => go(8)} />
      case 8:
        return <Execution onNext={() => go(9)} />
      case 9:
        return <Evidence onNext={() => go(10)} />
      case 10:
        return <AuditPack onRestart={() => go(0)} />
      default:
        return null
    }
  }

  return (
    <div className="flex h-full flex-col">
      <StageRail step={step} />
      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto px-6 py-6">
        {/* Keyed remount plays each screen's entrance; no exit-wait wrapper so a
            step change can never get stuck mid-transition. */}
        <motion.div
          key={step}
          initial={reduce ? false : { opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, ease: "easeOut" }}
        >
          {screen()}
        </motion.div>
      </div>
    </div>
  )
}
