import { NextRequest, NextResponse } from 'next/server';
import { getBackendUrl } from '@/lib/api-config';

const API_BASE_URL = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    // Используем специальный эндпоинт для дашборда
    const backendResponse = await fetch(`${API_BASE_URL}/api/dashboard/normalization-status`, {
      cache: 'no-store',
      headers: {
        'Content-Type': 'application/json',
      },
    });
    
    if (!backendResponse.ok) {
      return NextResponse.json(
        { error: 'Failed to fetch normalization status' },
        { status: backendResponse.status }
      );
    }
    
    const data = await backendResponse.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error('Error fetching normalization status:', error);
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    );
  }
}
