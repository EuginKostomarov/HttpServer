import { z } from 'zod'

/**
 * Validation schemas for API routes
 * Provides type-safe request validation using Zod
 */

// KPVED Load Request
export const kpvedLoadSchema = z.object({
  file_path: z.string().min(1, 'File path is required').optional(),
  force_reload: z.boolean().optional(),
}).passthrough() // Allow additional properties for flexibility

// KPVED Reclassify Hierarchical Request
export const kpvedReclassifySchema = z.object({
  database: z.string().min(1, 'Database name is required').optional(),
  batch_size: z.number().int().positive().optional(),
  max_workers: z.number().int().positive().max(100).optional(),
}).passthrough()

// Quality Analysis Request
export const qualityAnalyzeSchema = z.object({
  database: z.string().min(1, 'Database name is required').optional(),
  check_duplicates: z.boolean().optional(),
  check_violations: z.boolean().optional(),
  check_suggestions: z.boolean().optional(),
}).passthrough()

// Violation Resolution Request
export const violationResolveSchema = z.object({
  action: z.enum(['ignore', 'fix', 'defer']).optional(),
  notes: z.string().max(1000).optional(),
}).passthrough()

// Suggestion Application Request
export const suggestionApplySchema = z.object({
  auto_apply: z.boolean().optional(),
}).passthrough()

/**
 * Validation helper that returns standardized error response
 */
export function validateRequest<T>(
  schema: z.ZodSchema<T>,
  data: unknown
): { success: true; data: T } | { success: false; error: string; details: z.ZodError } {
  const result = schema.safeParse(data)

  if (!result.success) {
    return {
      success: false,
      error: 'Invalid request data',
      details: result.error,
    }
  }

  return {
    success: true,
    data: result.data,
  }
}

/**
 * Formats Zod validation errors for user-friendly display
 */
export function formatValidationError(error: z.ZodError): string {
  const messages = error.errors.map(err => {
    const path = err.path.join('.')
    return path ? `${path}: ${err.message}` : err.message
  })
  return messages.join('; ')
}
