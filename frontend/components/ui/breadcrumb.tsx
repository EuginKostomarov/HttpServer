import * as React from "react"
import { ChevronRight, Home } from "lucide-react"
import Link from "next/link"
import { cn } from "@/lib/utils"

export interface BreadcrumbItem {
  label: string
  href?: string
  icon?: React.ComponentType<{ className?: string }>
}

interface BreadcrumbProps extends React.HTMLAttributes<HTMLElement> {
  items: BreadcrumbItem[]
  homeHref?: string
  homeLabel?: string
  showHome?: boolean
}

const Breadcrumb = React.forwardRef<HTMLElement, BreadcrumbProps>(
  ({ className, items, homeHref = "/", homeLabel = "Главная", showHome = true, ...props }, ref) => {
    const allItems = showHome
      ? [{ label: homeLabel, href: homeHref }, ...items]
      : items

    return (
      <nav
        ref={ref}
        aria-label="Breadcrumb"
        className={cn("flex items-center space-x-1 text-sm", className)}
        {...props}
      >
        <ol className="flex items-center space-x-1.5">
          {allItems.map((item, index) => {
            const isLast = index === allItems.length - 1
            return (
              <li key={index} className="flex items-center">
                {index > 0 && (
                  <ChevronRight className="h-3.5 w-3.5 mx-1.5 text-muted-foreground/50" aria-hidden="true" />
                )}
                {item.href && !isLast ? (
                  <Link
                    href={item.href}
                    className="hover:text-foreground transition-all flex items-center gap-1.5 group px-2 py-1 rounded-md hover:bg-accent/50"
                    aria-current={isLast ? "page" : undefined}
                  >
                    {item.icon && (
                      <item.icon className="h-4 w-4 text-muted-foreground group-hover:text-foreground transition-colors" />
                    )}
                    <span className="text-muted-foreground group-hover:text-foreground transition-colors">{item.label}</span>
                  </Link>
                ) : (
                  <span
                    className={cn(
                      "font-semibold flex items-center gap-1.5 px-2 py-1 rounded-md bg-accent/30",
                      isLast && "text-foreground"
                    )}
                    aria-current={isLast ? "page" : undefined}
                  >
                    {item.icon && (
                      <item.icon className="h-4 w-4 text-primary" />
                    )}
                    <span>{item.label}</span>
                  </span>
                )}
              </li>
            )
          })}
        </ol>
      </nav>
    )
  }
)
Breadcrumb.displayName = "Breadcrumb"

export { Breadcrumb }

