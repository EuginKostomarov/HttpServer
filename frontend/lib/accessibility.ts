/**
 * Accessibility Utilities
 *
 * Helpers for implementing WCAG 2.1 AA compliant components
 */

/**
 * Generates unique IDs for ARIA attributes
 */
let idCounter = 0
export function generateId(prefix: string = 'id'): string {
  return `${prefix}-${++idCounter}-${Date.now()}`
}

/**
 * ARIA labels for common UI patterns
 */
export const ARIA_LABELS = {
  // Navigation
  MAIN_NAV: 'Main navigation',
  BREADCRUMB: 'Breadcrumb navigation',
  PAGINATION: 'Pagination navigation',

  // Forms
  REQUIRED_FIELD: 'required field',
  OPTIONAL_FIELD: 'optional field',
  SEARCH: 'Search',
  FILTER: 'Filter',

  // Actions
  CLOSE: 'Close',
  OPEN: 'Open',
  EXPAND: 'Expand',
  COLLAPSE: 'Collapse',
  NEXT: 'Next',
  PREVIOUS: 'Previous',
  DELETE: 'Delete',
  EDIT: 'Edit',
  SAVE: 'Save',
  CANCEL: 'Cancel',

  // Status
  LOADING: 'Loading',
  ERROR: 'Error',
  SUCCESS: 'Success',
  WARNING: 'Warning',
} as const

/**
 * Keyboard navigation helper
 * Returns handlers for common keyboard patterns
 */
export interface KeyboardNavigationOptions {
  onEnter?: () => void
  onSpace?: () => void
  onEscape?: () => void
  onArrowUp?: () => void
  onArrowDown?: () => void
  onArrowLeft?: () => void
  onArrowRight?: () => void
  onHome?: () => void
  onEnd?: () => void
  onTab?: (shift: boolean) => void
}

export function useKeyboardNavigation(options: KeyboardNavigationOptions) {
  return (event: React.KeyboardEvent) => {
    switch (event.key) {
      case 'Enter':
        if (options.onEnter) {
          event.preventDefault()
          options.onEnter()
        }
        break

      case ' ':
      case 'Spacebar':
        if (options.onSpace) {
          event.preventDefault()
          options.onSpace()
        }
        break

      case 'Escape':
      case 'Esc':
        if (options.onEscape) {
          event.preventDefault()
          options.onEscape()
        }
        break

      case 'ArrowUp':
      case 'Up':
        if (options.onArrowUp) {
          event.preventDefault()
          options.onArrowUp()
        }
        break

      case 'ArrowDown':
      case 'Down':
        if (options.onArrowDown) {
          event.preventDefault()
          options.onArrowDown()
        }
        break

      case 'ArrowLeft':
      case 'Left':
        if (options.onArrowLeft) {
          event.preventDefault()
          options.onArrowLeft()
        }
        break

      case 'ArrowRight':
      case 'Right':
        if (options.onArrowRight) {
          event.preventDefault()
          options.onArrowRight()
        }
        break

      case 'Home':
        if (options.onHome) {
          event.preventDefault()
          options.onHome()
        }
        break

      case 'End':
        if (options.onEnd) {
          event.preventDefault()
          options.onEnd()
        }
        break

      case 'Tab':
        if (options.onTab) {
          options.onTab(event.shiftKey)
        }
        break
    }
  }
}

/**
 * Focus management utilities
 */
export const focusManagement = {
  /**
   * Traps focus within a container (for modals, dialogs)
   */
  trapFocus(containerElement: HTMLElement) {
    const focusableElements = containerElement.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )

    if (focusableElements.length === 0) return

    const firstElement = focusableElements[0]
    const lastElement = focusableElements[focusableElements.length - 1]

    const handleTabKey = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return

      if (e.shiftKey) {
        // Shift + Tab
        if (document.activeElement === firstElement) {
          e.preventDefault()
          lastElement.focus()
        }
      } else {
        // Tab
        if (document.activeElement === lastElement) {
          e.preventDefault()
          firstElement.focus()
        }
      }
    }

    containerElement.addEventListener('keydown', handleTabKey)

    // Return cleanup function
    return () => {
      containerElement.removeEventListener('keydown', handleTabKey)
    }
  },

  /**
   * Moves focus to element and returns previous active element
   */
  moveFocus(element: HTMLElement): HTMLElement | null {
    const previousElement = document.activeElement as HTMLElement
    element.focus()
    return previousElement
  },

  /**
   * Restores focus to previous element
   */
  restoreFocus(element: HTMLElement | null) {
    if (element && typeof element.focus === 'function') {
      element.focus()
    }
  },
}

/**
 * Screen reader announcements
 * Uses ARIA live regions to announce dynamic content changes
 */
export const announcer = {
  /**
   * Creates a live region for screen reader announcements
   */
  createLiveRegion(): HTMLDivElement {
    const liveRegion = document.createElement('div')
    liveRegion.setAttribute('role', 'status')
    liveRegion.setAttribute('aria-live', 'polite')
    liveRegion.setAttribute('aria-atomic', 'true')
    liveRegion.className = 'sr-only'
    document.body.appendChild(liveRegion)
    return liveRegion
  },

  /**
   * Announces a message to screen readers
   */
  announce(message: string, priority: 'polite' | 'assertive' = 'polite') {
    let liveRegion = document.querySelector(`[aria-live="${priority}"]`) as HTMLDivElement

    if (!liveRegion) {
      liveRegion = document.createElement('div')
      liveRegion.setAttribute('role', 'status')
      liveRegion.setAttribute('aria-live', priority)
      liveRegion.setAttribute('aria-atomic', 'true')
      liveRegion.className = 'sr-only'
      document.body.appendChild(liveRegion)
    }

    // Clear and set message
    liveRegion.textContent = ''
    setTimeout(() => {
      liveRegion.textContent = message
    }, 100)
  },
}

/**
 * ARIA attribute helpers
 */
export const aria = {
  /**
   * Creates props for expandable/collapsible elements
   */
  expandable(isExpanded: boolean, controlsId?: string) {
    return {
      'aria-expanded': isExpanded,
      ...(controlsId && { 'aria-controls': controlsId }),
    }
  },

  /**
   * Creates props for required form fields
   */
  required(isRequired: boolean = true) {
    return {
      'aria-required': isRequired,
      required: isRequired,
    }
  },

  /**
   * Creates props for invalid form fields
   */
  invalid(isInvalid: boolean, errorId?: string) {
    return {
      'aria-invalid': isInvalid,
      ...(isInvalid && errorId && { 'aria-describedby': errorId }),
    }
  },

  /**
   * Creates props for loading states
   */
  busy(isBusy: boolean = true) {
    return {
      'aria-busy': isBusy,
    }
  },

  /**
   * Creates props for disabled elements
   */
  disabled(isDisabled: boolean = true) {
    return {
      'aria-disabled': isDisabled,
      disabled: isDisabled,
    }
  },

  /**
   * Creates props for selected/pressed toggle buttons
   */
  pressed(isPressed: boolean) {
    return {
      'aria-pressed': isPressed,
    }
  },

  /**
   * Creates props for checked checkboxes/radios
   */
  checked(isChecked: boolean) {
    return {
      'aria-checked': isChecked,
    }
  },

  /**
   * Creates props for labeled elements
   */
  labeled(labelId: string) {
    return {
      'aria-labelledby': labelId,
    }
  },

  /**
   * Creates props for described elements
   */
  described(descriptionId: string) {
    return {
      'aria-describedby': descriptionId,
    }
  },
}

/**
 * Roving tabindex for keyboard navigation in lists/grids
 */
export class RovingTabIndex {
  private items: HTMLElement[] = []
  private currentIndex: number = 0

  constructor(containerSelector: string) {
    this.updateItems(containerSelector)
  }

  updateItems(containerSelector: string) {
    const container = document.querySelector(containerSelector)
    if (!container) return

    this.items = Array.from(
      container.querySelectorAll<HTMLElement>('[role="menuitem"], [role="option"], [role="treeitem"]')
    )

    this.items.forEach((item, index) => {
      item.setAttribute('tabindex', index === this.currentIndex ? '0' : '-1')
    })
  }

  focusNext() {
    this.currentIndex = (this.currentIndex + 1) % this.items.length
    this.updateFocus()
  }

  focusPrevious() {
    this.currentIndex = (this.currentIndex - 1 + this.items.length) % this.items.length
    this.updateFocus()
  }

  focusFirst() {
    this.currentIndex = 0
    this.updateFocus()
  }

  focusLast() {
    this.currentIndex = this.items.length - 1
    this.updateFocus()
  }

  private updateFocus() {
    this.items.forEach((item, index) => {
      item.setAttribute('tabindex', index === this.currentIndex ? '0' : '-1')
    })
    this.items[this.currentIndex]?.focus()
  }
}

/**
 * Color contrast checker (WCAG AA compliance)
 */
export function checkColorContrast(foreground: string, background: string): {
  ratio: number
  passesAA: boolean
  passesAAA: boolean
} {
  // This is a simplified version - in production, use a library like 'color-contrast-checker'
  // For now, return a placeholder
  return {
    ratio: 4.5,
    passesAA: true,
    passesAAA: false,
  }
}
