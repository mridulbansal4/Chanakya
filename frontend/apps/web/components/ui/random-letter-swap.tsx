"use client"

import * as React from "react"
import { motion } from "framer-motion"

const ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789#$%"

interface RandomLetterSwapProps {
  label: string
  className?: string
  staggerDuration?: number
  transition?: Record<string, unknown>
}

export function RandomLetterSwap({
  label,
  className = "",
  staggerDuration = 0.025,
  transition = { duration: 0.6, type: "spring" },
}: RandomLetterSwapProps) {
  const [displayText, setDisplayText] = React.useState(label)
  const [isHovered, setIsHovered] = React.useState(false)
  const intervalRef = React.useRef<number | null>(null)

  const triggerSwap = React.useCallback(() => {
    let iteration = 0
    if (intervalRef.current !== null) clearInterval(intervalRef.current)

    intervalRef.current = window.setInterval(() => {
      setDisplayText(
        label
          .split("")
          .map((char, index) => {
            if (char === " ") return " "
            if (index < iteration) {
              return label[index]
            }
            return ALPHABET[Math.floor(Math.random() * ALPHABET.length)]
          })
          .join("")
      )

      if (iteration >= label.length) {
        if (intervalRef.current !== null) clearInterval(intervalRef.current)
        setDisplayText(label)
      }

      iteration += 1 / 3
    }, Math.max(50, staggerDuration * 2000))
  }, [label, staggerDuration])

  const handleMouseEnter = () => {
    setIsHovered(true)
    triggerSwap()
  }

  const handleMouseLeave = () => {
    setIsHovered(false)
    if (intervalRef.current !== null) clearInterval(intervalRef.current)
    setDisplayText(label)
  }

  React.useEffect(() => {
    setDisplayText(label)
    return () => {
      if (intervalRef.current !== null) clearInterval(intervalRef.current)
    }
  }, [label])

  return (
    <motion.span
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      className={`inline-block whitespace-nowrap font-mono ${className}`}
      initial={false}
      animate={{ scale: isHovered ? 1.02 : 1 }}
      transition={transition}
    >
      {displayText}
    </motion.span>
  )
}
