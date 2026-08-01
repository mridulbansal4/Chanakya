import type { Transition, Variants } from "framer-motion"

/**
 * The single motion vocabulary for CHANAKYA.
 *
 * Every animation in the app draws from this file. The rule behind it:
 * motion exists to explain where a thing came from and where it went. If a
 * movement does not answer one of those two questions, it should not exist.
 *
 * Durations mirror the --dur-* tokens in globals.css. Keep them in sync.
 */

/** Micro — colour, opacity, hover. Below ~100ms reads as an instant snap. */
export const DUR_MICRO = 0.12
/** Standard — dropdowns, popovers, small enter/exit. */
export const DUR_STANDARD = 0.18
/** Structural — page and dialog transitions, layout changes. */
export const DUR_STRUCTURAL = 0.26

/** Fast departure, soft arrival. Matches --ease-out. */
export const EASE_OUT = [0.2, 0.8, 0.2, 1] as const
export const EASE_IN_OUT = [0.65, 0, 0.35, 1] as const

/**
 * Springs are reserved for things the user is directly manipulating or
 * that physically move between two positions — a layout indicator sliding
 * to a new tab, a panel being dragged. Everything else uses a duration
 * curve, which is more predictable and cheaper.
 */
export const SPRING_LAYOUT: Transition = {
  type: "spring",
  stiffness: 400,
  damping: 34,
  mass: 0.7,
}

export const SPRING_SNAPPY: Transition = {
  type: "spring",
  stiffness: 520,
  damping: 38,
  mass: 0.6,
}

export const TRANSITION_MICRO: Transition = {
  duration: DUR_MICRO,
  ease: EASE_OUT,
}

export const TRANSITION_STANDARD: Transition = {
  duration: DUR_STANDARD,
  ease: EASE_OUT,
}

export const TRANSITION_STRUCTURAL: Transition = {
  duration: DUR_STRUCTURAL,
  ease: EASE_OUT,
}

/**
 * Page-level enter. Deliberately small: a 4px rise reads as the content
 * settling into place. Anything larger reads as a slide and makes rapid
 * navigation feel sluggish.
 */
export const pageVariants: Variants = {
  hidden: { opacity: 0, y: 4 },
  visible: { opacity: 1, y: 0 },
}

/**
 * Overlay entry for dialogs and the command palette. Scale starts at 0.98,
 * not 0.94 — a large scale jump on a full-width panel reads as a zoom
 * effect rather than an appearance.
 */
export const overlayVariants: Variants = {
  hidden: { opacity: 0, scale: 0.98, y: -6 },
  visible: { opacity: 1, scale: 1, y: 0 },
  exit: { opacity: 0, scale: 0.98, y: -6 },
}

export const scrimVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1 },
  exit: { opacity: 0 },
}

/**
 * Staggered list entry, capped.
 *
 * An uncapped `delay: index * step` means the 40th row in a table waits two
 * full seconds before appearing — the stagger stops being a flourish and
 * becomes latency. This clamps total stagger time regardless of list length.
 */
export function staggerDelay(index: number, step = 0.03, cap = 0.24): number {
  return Math.min(index * step, cap)
}

/**
 * Container/child pair for list reveals where the parent orchestrates.
 */
export const listContainer: Variants = {
  hidden: {},
  visible: {
    transition: { staggerChildren: 0.03, delayChildren: 0.02 },
  },
}

export const listItem: Variants = {
  hidden: { opacity: 0, y: 6 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: DUR_STANDARD, ease: EASE_OUT },
  },
}
