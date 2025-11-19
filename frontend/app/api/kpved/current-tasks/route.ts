import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://127.0.0.1:9999'

export async function GET(_request: NextRequest) {
  try {
    const response = await fetch(`${BACKEND_URL}/api/kpved/current-tasks`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      return NextResponse.json(
        { error: `Backend responded with ${response.status}` },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching KPVED current tasks:', error)
    return NextResponse.json(
      {
        current_tasks: [],
        count: 0,
        error: 'Не удалось получить текущие задачи',
      },
      { status: 500 }
    )
  }
}

