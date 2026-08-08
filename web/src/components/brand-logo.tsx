import type { SVGProps } from "react"

import { cn } from "@/lib/utils"

type BrandMarkProps = SVGProps<SVGSVGElement> & {
  title?: string
}

/**
 * The Sub Manager mark is an S-shaped route with three connection points.
 * Keep this component in sync with public/brand/logo-mark.svg.
 */
export function BrandMark({ className, title, ...props }: BrandMarkProps) {
  return (
    <svg
      viewBox="0 0 64 64"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={cn("shrink-0", className)}
      role={title ? "img" : undefined}
      aria-hidden={title ? undefined : true}
      {...props}
    >
      {title ? <title>{title}</title> : null}
      <rect x="3" y="3" width="58" height="58" rx="17" fill="#0F172A" />
      <rect
        x="3.75"
        y="3.75"
        width="56.5"
        height="56.5"
        rx="16.25"
        stroke="white"
        strokeOpacity="0.12"
        strokeWidth="1.5"
      />
      <path
        d="M44.5 18.5C41.25 14.9 35.55 13.75 30.35 14.7C24.35 15.8 20 19.05 20 23.7C20 29.8 25.75 31.35 32.25 33.05C38.35 34.65 44 36.4 44 42.1C44 47.35 39.15 50.3 32.75 50.3C26.8 50.3 21.75 48 18.8 44.45"
        stroke="#5EEAD4"
        strokeWidth="5.5"
        strokeLinecap="round"
      />
      <circle cx="44.5" cy="18.5" r="4" fill="#F8FAFC" />
      <circle cx="31.95" cy="33" r="3.25" fill="#F8FAFC" />
      <circle cx="18.8" cy="44.45" r="4" fill="#F8FAFC" />
    </svg>
  )
}

type BrandLockupProps = {
  className?: string
  compact?: boolean
  heading?: boolean
}

export function BrandLockup({
  className,
  compact = false,
  heading = false,
}: BrandLockupProps) {
  const Name = heading ? "h1" : "div"

  return (
    <div className={cn("flex min-w-0 items-center gap-3", className)}>
      <BrandMark className={compact ? "size-9" : "size-11"} />
      <div className="min-w-0 leading-none">
        <Name
          className={cn(
            "truncate font-heading font-semibold tracking-[-0.025em]",
            compact ? "text-base" : "text-lg"
          )}
        >
          Sub Manager
        </Name>
        <div className="mt-1 truncate text-[0.6875rem] font-medium tracking-[0.08em] text-muted-foreground uppercase">
          Xray · Mihomo
        </div>
      </div>
    </div>
  )
}
