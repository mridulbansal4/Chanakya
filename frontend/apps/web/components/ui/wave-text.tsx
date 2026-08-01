"use client"

import { motion } from "framer-motion"
import { cn } from "@workspace/ui/lib/utils"

interface WaveTextProps {
  text: string
  className?: string
}

export function WaveText({ text, className = "" }: WaveTextProps) {
  return (
    <motion.span
      className={cn("inline-flex items-center cursor-pointer select-none", className)}
      whileHover="hover"
      initial="initial"
    >
      {text.split("").map((char, index) => (
        <motion.span
          key={`${char}-${index}`}
          className="inline-block"
          variants={{
            initial: {
              y: 0,
              scale: 1,
            },
            hover: {
              y: -3,
              scale: 1.15,
              transition: {
                type: "spring",
                stiffness: 350,
                damping: 18,
                delay: index * 0.025,
              },
            },
          }}
        >
          {char === " " ? "\u00A0" : char}
        </motion.span>
      ))}
    </motion.span>
  )
}
