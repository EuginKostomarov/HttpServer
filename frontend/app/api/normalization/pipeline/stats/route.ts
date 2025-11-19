import { NextRequest, NextResponse } from 'next/server';

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:9999';

export async function GET(request: NextRequest) {
  try {
    const backendURL = `${BACKEND_URL}/api/normalization/pipeline/stats`;

    const backendResponse = await fetch(backendURL, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      // Disable caching for real-time data
      cache: 'no-store',
    });

    if (!backendResponse.ok) {
      const errorText = await backendResponse.text();
      console.error(`Backend error (${backendResponse.status}):`, errorText);

      return NextResponse.json(
        {
          error: 'Failed to fetch pipeline stats',
          details: errorText,
          status: backendResponse.status
        },
        { status: backendResponse.status }
      );
    }

    const data = await backendResponse.json();

    return NextResponse.json(data, {
      headers: {
        'Cache-Control': 'no-store, no-cache, must-revalidate',
      },
    });
  } catch (error) {
    console.error('Pipeline stats API error:', error);

    return NextResponse.json(
      {
        error: 'Internal server error',
        message: error instanceof Error ? error.message : 'Unknown error'
      },
      { status: 500 }
    );
  }
}
