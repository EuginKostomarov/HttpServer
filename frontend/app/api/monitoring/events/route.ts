import { NextRequest } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    // Создаем ReadableStream для проксирования SSE
    const stream = new ReadableStream({
      async start(controller) {
        try {
          // Делаем запрос к бэкенду
          const response = await fetch(`${BACKEND_URL}/api/monitoring/events`, {
            headers: {
              'Cache-Control': 'no-cache',
            },
          })

          if (!response.ok) {
            controller.enqueue(
              new TextEncoder().encode(`data: ${JSON.stringify({ type: 'error', message: 'Failed to connect to backend' })}\n\n`)
            )
            controller.close()
            return
          }

          // Читаем поток от бэкенда и передаем клиенту
          const reader = response.body?.getReader()
          const decoder = new TextDecoder()

          if (!reader) {
            controller.close()
            return
          }

          while (true) {
            const { done, value } = await reader.read()
            
            if (done) {
              controller.close()
              break
            }

            // Передаем данные клиенту
            controller.enqueue(value)
          }
        } catch (error) {
          console.error('Error proxying SSE:', error)
          controller.enqueue(
            new TextEncoder().encode(`data: ${JSON.stringify({ type: 'error', message: error instanceof Error ? error.message : 'Unknown error' })}\n\n`)
          )
          controller.close()
        }
      },
    })

    // Возвращаем Response с SSE заголовками
    return new Response(stream, {
      headers: {
        'Content-Type': 'text/event-stream',
        'Cache-Control': 'no-cache',
        'Connection': 'keep-alive',
        'Access-Control-Allow-Origin': '*',
        'Access-Control-Allow-Headers': 'Cache-Control',
      },
    })
  } catch (error) {
    console.error('Error creating SSE connection:', error)
    return new Response(
      JSON.stringify({ error: 'Failed to establish SSE connection' }),
      {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      }
    )
  }
}

