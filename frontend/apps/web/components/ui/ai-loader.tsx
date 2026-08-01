"use client"

import { motion } from "framer-motion"
import { cn } from "@workspace/ui/lib/utils"

interface AiLoaderProps {
  text?: string
  className?: string
}

export function AiLoader({ text = "Generating", className }: AiLoaderProps) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-4", className)}>
      <div className="flex items-center gap-[1px]">
        {text.split("").map((char, index) => (
          <motion.span
            key={`${char}-${index}`}
            className="inline-block text-accent font-mono text-sm font-bold tracking-widest uppercase"
            animate={{
              y: [0, -4, 0],
              opacity: [0.3, 1, 0.3],
            }}
            transition={{
              duration: 1.5,
              repeat: Infinity,
              delay: index * 0.1,
              ease: "easeInOut",
            }}
          >
            {char === " " ? "\u00A0" : char}
          </motion.span>
        ))}
      </div>
      
      {/* Crazy looking circle planet loader */}
      <div className="relative flex items-center justify-center size-20 mt-2 mb-2">
        {/* Glowing Planet Core */}
        <div className="absolute size-7 rounded-full bg-gradient-to-tr from-accent to-blue-400 shadow-[0_0_24px_var(--accent)] opacity-90" />
        
        {/* Outer Orbiting Ring 1 */}
        <motion.div
          className="absolute size-14 rounded-full border-[1.5px] border-accent/20 border-t-accent"
          style={{ transformStyle: "preserve-3d" }}
          animate={{ rotateX: 70, rotateZ: 360 }}
          transition={{ duration: 1.5, repeat: Infinity, ease: "linear" }}
        />
        
        {/* Outer Orbiting Ring 2 */}
        <motion.div
          className="absolute size-20 rounded-full border-[1.5px] border-accent/10 border-l-accent"
          style={{ transformStyle: "preserve-3d" }}
          animate={{ rotateX: 65, rotateY: 55, rotateZ: -360 }}
          transition={{ duration: 2.5, repeat: Infinity, ease: "linear" }}
        />
        
        {/* Orbiting Moon */}
        <motion.div
          className="absolute size-14"
          style={{ transformStyle: "preserve-3d" }}
          animate={{ rotateX: 70, rotateZ: 360 }}
          transition={{ duration: 1.5, repeat: Infinity, ease: "linear" }}
        >
          <div className="absolute top-0 left-1/2 -ml-1 size-2 rounded-full bg-white shadow-[0_0_8px_white]" />
        </motion.div>
      </div>
    </div>
  )
}
