import { NextRequest, NextResponse } from 'next/server';
import { getBackendUrl } from '@/lib/api-config';

const API_BASE_URL = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    const backendResponse = await fetch(`${API_BASE_URL}/api/dashboard/stats`, {
      cache: 'no-store',
      headers: {
        'Content-Type': 'application/json',
      },
    });
    
    if (!backendResponse.ok) {
      const { ApiErrorHandler } = await import('@/lib/api-error-handler-enhanced')
      return ApiErrorHandler.handleErrorWithEnhancement(
        backendResponse,
        '/api/dashboard/stats'
      )
    }
    
    const data = await backendResponse.json();
    return NextResponse.json(data);
  } catch (error) {
    const { ApiErrorHandler } = await import('@/lib/api-error-handler-enhanced')
    ApiErrorHandler.logError('/api/dashboard/stats', error)
    return ApiErrorHandler.createErrorResponse(error, 'Failed to fetch dashboard stats')
  }
}
