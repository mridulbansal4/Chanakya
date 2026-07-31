import * as React from "react"
import { Button as ButtonPrimitive } from "@base-ui/react/button"
import { cva, type VariantProps } from "class-variance-authority"
import { Check, Loader2 } from "lucide-react"

import { cn } from "@workspace/ui/lib/utils"

const buttonVariants = cva(
  "group/button relative inline-flex shrink-0 items-center justify-center rounded-xl font-medium whitespace-nowrap transition-all duration-200 ease-out outline-none select-none active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4 [&_svg]:transition-transform [&_svg]:duration-200 group-hover/button:[&_svg]:scale-105",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-sm hover:bg-primary/90 hover:shadow-md hover:-translate-y-0.5 border border-primary/20 active:translate-y-0",
        outline:
          "border border-border bg-surface text-foreground shadow-xs hover:bg-surface-2 hover:border-foreground/30 hover:-translate-y-0.5 active:translate-y-0",
        secondary:
          "bg-cream-200 text-foreground hover:bg-cream-200/80 hover:-translate-y-0.5 active:translate-y-0 border border-line/60",
        ghost:
          "text-foreground hover:bg-cream-200/60 hover:text-foreground",
        destructive:
          "bg-destructive text-destructive-foreground shadow-xs hover:bg-destructive/90 hover:shadow-md hover:-translate-y-0.5 active:translate-y-0",
        success:
          "bg-ok text-white shadow-xs hover:bg-ok/90 hover:shadow-md hover:-translate-y-0.5 active:translate-y-0",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-4 py-2 text-sm gap-2",
        xs: "h-7 px-2.5 text-xs gap-1.5 rounded-lg [&_svg:not([class*='size-'])]:size-3",
        sm: "h-8 px-3 text-xs gap-1.5 rounded-lg [&_svg:not([class*='size-'])]:size-3.5",
        lg: "h-11 px-6 text-base gap-2.5 rounded-2xl [&_svg:not([class*='size-'])]:size-5",
        icon: "size-9 rounded-xl",
        "icon-xs": "size-7 rounded-lg [&_svg:not([class*='size-'])]:size-3.5",
        "icon-sm": "size-8 rounded-lg [&_svg:not([class*='size-'])]:size-4",
        "icon-lg": "size-11 rounded-2xl [&_svg:not([class*='size-'])]:size-5",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

export interface ButtonProps
  extends ButtonPrimitive.Props,
    VariantProps<typeof buttonVariants> {
  isLoading?: boolean
  isSuccess?: boolean
  loadingText?: string
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant,
      size,
      isLoading = false,
      isSuccess = false,
      loadingText,
      children,
      disabled,
      ...props
    },
    ref
  ) => {
    return (
      <ButtonPrimitive
        ref={ref}
        data-slot="button"
        disabled={disabled || isLoading}
        className={cn(buttonVariants({ variant, size, className }))}
        {...props}
      >
        {isLoading ? (
          <>
            <Loader2 className="size-4 animate-spin text-current" />
            <span>{loadingText ?? children}</span>
          </>
        ) : isSuccess ? (
          <>
            <Check className="size-4 text-white animate-in zoom-in-75 duration-150" />
            <span>{children}</span>
          </>
        ) : (
          children
        )}
      </ButtonPrimitive>
    )
  }
)

Button.displayName = "Button"

export { Button, buttonVariants }

