"use client"

import * as React from "react"
import { motion, useReducedMotion } from "framer-motion"

import { TRANSITION_STRUCTURAL, pageVariants } from "@/lib/motion"

/**
 * Route-level enter animation.
 *
 * The movement is 4px. On a screen the user navigates between all day, a
 * larger travel distance stops reading as polish and starts reading as
 * waiting — the animation becomes the slowest part of the navigation.
 */
export function PageTransition({ children }: { children: React.ReactNode }) {
  const reduce = useReducedMotion()

  if (reduce) {
    return <div className="flex h-full w-full flex-col">{children}</div>
  }

  return (
    <motion.div
      initial="hidden"
      animate="visible"
      variants={pageVariants}
      transition={TRANSITION_STRUCTURAL}
      className="flex h-full w-full flex-col"
    >
      {children}
    </motion.div>
  )
}
