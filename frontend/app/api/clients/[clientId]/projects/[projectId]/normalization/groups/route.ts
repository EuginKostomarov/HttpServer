import { NextRequest, NextResponse } from 'next/server';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080';

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ clientId: string; projectId: string }> }
) {
  try {
    const { clientId, projectId } = await params;
    const { searchParams } = new URL(request.url);
    
    const dbId = searchParams.get('db_id');
    const page = searchParams.get('page') || '1';
    const limit = searchParams.get('limit') || '20';

    if (!dbId) {
      return NextResponse.json(
        { error: 'db_id parameter is required' },
        { status: 400 }
      );
    }

    const url = new URL(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/normalization/groups`);
    url.searchParams.set('db_id', dbId);
    url.searchParams.set('page', page);
    url.searchParams.set('limit', limit);

    const response = await fetch(url.toString(), {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({ error: 'Failed to fetch normalization groups' }));
      return NextResponse.json(
        { error: errorData.error || 'Failed to fetch normalization groups' },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error('Error fetching normalization groups:', error);
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    );
  }
}

