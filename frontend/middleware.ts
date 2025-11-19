import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

/**
 * Security Middleware for API Routes
 *
 * Provides:
 * - API key authentication for production environments
 * - Rate limiting headers
 * - Security headers (CORS, CSP, etc.)
 */

// API routes that should be protected
const PROTECTED_API_ROUTES = [
  '/api/kpved',
  '/api/quality',
  '/api/normalization',
  '/api/classifiers',
  '/api/workers',
]

// Check if request path matches protected routes
function isProtectedRoute(pathname: string): boolean {
  return PROTECTED_API_ROUTES.some(route => pathname.startsWith(route))
}

// Simple rate limiting (in-memory, for demo - use Redis in production)
const rateLimitMap = new Map<string, { count: number; resetTime: number }>()
const RATE_LIMIT_WINDOW = 60 * 1000 // 1 minute
const RATE_LIMIT_MAX_REQUESTS = 100 // 100 requests per minute

function checkRateLimit(identifier: string): { allowed: boolean; remaining: number } {
  const now = Date.now()
  const record = rateLimitMap.get(identifier)

  if (!record || now > record.resetTime) {
    // Reset or create new record
    rateLimitMap.set(identifier, {
      count: 1,
      resetTime: now + RATE_LIMIT_WINDOW,
    })
    return { allowed: true, remaining: RATE_LIMIT_MAX_REQUESTS - 1 }
  }

  if (record.count >= RATE_LIMIT_MAX_REQUESTS) {
    return { allowed: false, remaining: 0 }
  }

  record.count++
  return { allowed: true, remaining: RATE_LIMIT_MAX_REQUESTS - record.count }
}

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl

  // Only process API routes
  if (!pathname.startsWith('/api/')) {
    return NextResponse.next()
  }

  // Get client identifier (IP address or API key)
  const forwarded = request.headers.get('x-forwarded-for')
  const ip = forwarded ? forwarded.split(',')[0] : request.ip || 'unknown'

  // Check rate limit
  const rateLimit = checkRateLimit(ip)

  if (!rateLimit.allowed) {
    return NextResponse.json(
      { error: 'Too many requests. Please try again later.' },
      {
        status: 429,
        headers: {
          'Retry-After': '60',
          'X-RateLimit-Limit': RATE_LIMIT_MAX_REQUESTS.toString(),
          'X-RateLimit-Remaining': '0',
          'X-RateLimit-Reset': new Date(Date.now() + RATE_LIMIT_WINDOW).toISOString(),
        }
      }
    )
  }

  // API Key Authentication for protected routes in production
  if (isProtectedRoute(pathname)) {
    const apiKey = request.headers.get('x-api-key') || request.headers.get('authorization')?.replace('Bearer ', '')
    const expectedApiKey = process.env.API_KEY

    // Only enforce API key in production if configured
    if (process.env.NODE_ENV === 'production' && expectedApiKey) {
      if (!apiKey || apiKey !== expectedApiKey) {
        return NextResponse.json(
          { error: 'Unauthorized. Invalid or missing API key.' },
          {
            status: 401,
            headers: {
              'WWW-Authenticate': 'Bearer realm="API"',
            }
          }
        )
      }
    }
  }

  // Create response with security headers
  const response = NextResponse.next()

  // Security headers
  response.headers.set('X-Content-Type-Options', 'nosniff')
  response.headers.set('X-Frame-Options', 'DENY')
  response.headers.set('X-XSS-Protection', '1; mode=block')
  response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin')

  // Rate limit headers
  response.headers.set('X-RateLimit-Limit', RATE_LIMIT_MAX_REQUESTS.toString())
  response.headers.set('X-RateLimit-Remaining', rateLimit.remaining.toString())

  // CORS headers for API routes (if needed)
  if (pathname.startsWith('/api/')) {
    const origin = request.headers.get('origin')
    // Only allow same-origin or configured origins
    if (origin === request.nextUrl.origin || process.env.ALLOWED_ORIGINS?.includes(origin || '')) {
      response.headers.set('Access-Control-Allow-Origin', origin || '*')
      response.headers.set('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS')
      response.headers.set('Access-Control-Allow-Headers', 'Content-Type, Authorization, X-API-Key')
      response.headers.set('Access-Control-Max-Age', '86400')
    }
  }

  return response
}

// Configure which routes the middleware runs on
export const config = {
  matcher: [
    /*
     * Match all API routes
     */
    '/api/:path*',
  ],
}
