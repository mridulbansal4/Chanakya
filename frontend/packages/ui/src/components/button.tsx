import * as React from "react"
import { Button as ButtonPrimitive } from "@base-ui/react/button"
import { cva, type VariantProps } from "class-variance-authority"
import { Check, Loader2 } from "lucide-react"

import { cn } from "@workspace/ui/lib/utils"

/**
 * Buttons do not translate on hover.
 *
 * A -2px lift on every button means the whole interface twitches as the
 * pointer crosses it, and on a dense screen that reads as instability. The
 * hover signal here is a fill change plus a border change, which is legible
 * without moving anything. Press is the only transform, and it moves *into*
 * the surface, which is what a physical control does.
 *
 * Sizing: `default` is 36px and `lg` is 44px. Any button that is a primary
 * touch target on a small screen should use `lg` to clear the 44px minimum.
 */
const buttonVariants = cva(
  [
    "group/button relative inline-flex shrink-0 items-center justify-center",
    "font-sans font-medium whitespace-nowrap select-none",
    "shiny-cta",
    "transition-[background-color,border-color,box-shadow,color] duration-[120ms] ease-[cubic-bezier(0.2,0.8,0.2,1)]",
    "active:scale-[0.985]",
    "disabled:pointer-events-none disabled:opacity-45",
    "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  ].join(" "),
  {
    variants: {
      variant: {
        /** The one high-emphasis action on a screen. There should rarely be two. */
        default:
          "border-transparent bg-accent-solid text-accent-on shadow-elev-1 hover:bg-accent-hover",
        /** Default for most actions: reads as a control, not a call to action. */
        outline:
          "border-line bg-raised text-fg hover:bg-elevated hover:border-line-strong",
        secondary:
          "border-line-subtle bg-elevated text-fg hover:border-line-strong hover:bg-line-subtle",
        /** Lowest emphasis. No border until hovered, so it recedes into the surface. */
        ghost:
          "border-transparent bg-transparent text-fg-muted hover:bg-elevated hover:text-fg",
        destructive:
          "border-transparent bg-risk text-white shadow-elev-1 hover:brightness-110",
        success:
          "border-transparent bg-ok text-white shadow-elev-1 hover:brightness-110",
        link: "border-transparent bg-transparent text-accent underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 gap-2 px-3.5 text-[0.875rem]",
        xs: "h-7 gap-1.5 rounded px-2 text-[0.75rem] [&_svg:not([class*='size-'])]:size-3",
        sm: "h-8 gap-1.5 px-3 text-[0.8125rem] [&_svg:not([class*='size-'])]:size-3.5",
        /** 44px - the minimum comfortable touch target. */
        lg: "h-11 gap-2.5 px-5 text-[0.9375rem]",
        icon: "size-9",
        "icon-xs": "size-7 rounded [&_svg:not([class*='size-'])]:size-3.5",
        "icon-sm": "size-8 [&_svg:not([class*='size-'])]:size-4",
        "icon-lg": "size-11 [&_svg:not([class*='size-'])]:size-5",
      },
    },
    defaultVariants: {
      variant: "outline",
      size: "default",
    },
  },
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
    ref,
  ) => {
    return (
      <ButtonPrimitive
        ref={ref}
        data-slot="button"
        disabled={disabled || isLoading}
        /* Announce the pending state rather than only showing a spinner -
           a screen reader user otherwise gets no feedback that the click
           was received. */
        aria-busy={isLoading || undefined}
        className={cn(buttonVariants({ variant, size, className }))}
        {...props}
      >
        <span>
          {isLoading ? (
            <>
              <Loader2 className="size-4 animate-spin" aria-hidden />
              <span>{loadingText ?? children}</span>
            </>
          ) : isSuccess ? (
            <>
              <Check className="size-4" aria-hidden />
              <span>{children}</span>
            </>
          ) : (
            children
          )}
        </span>
      </ButtonPrimitive>
    )
  },
)

Button.displayName = "Button"

export { Button, buttonVariants }
