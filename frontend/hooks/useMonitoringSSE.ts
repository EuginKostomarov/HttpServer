import { useEffect, useRef, useState } from 'react'

interface SSEMetrics {
  type: string
  timestamp: string
  uptime_seconds: number
  throughput: number
  ai_success_rate: number
  cache_hit_rate: number
  batch_queue_size: number
  circuit_breaker_state: string
  checkpoint_progress: number
}

export function useMonitoringSSE(enabled: boolean = true) {
  const [metrics, setMetrics] = useState<SSEMetrics | null>(null)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const eventSourceRef = useRef<EventSource | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const reconnectAttemptsRef = useRef(0)

  useEffect(() => {
    if (!enabled) {
      // Disconnect if disabled
      if (eventSourceRef.current) {
        eventSourceRef.current.close()
        eventSourceRef.current = null
        setConnected(false)
      }
      return
    }

    const connect = () => {
      try {
        // Close existing connection if any
        if (eventSourceRef.current) {
          eventSourceRef.current.close()
        }

        // Create new EventSource
        const eventSource = new EventSource('/api/monitoring/events')
        eventSourceRef.current = eventSource

        eventSource.onopen = () => {
          console.log('[SSE] Connected to monitoring events')
          setConnected(true)
          setError(null)
          reconnectAttemptsRef.current = 0
        }

        eventSource.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data)

            if (data.type === 'connected') {
              console.log('[SSE] Connection confirmed:', data.message)
            } else if (data.type === 'metrics') {
              setMetrics(data)
            }
          } catch (err) {
            console.error('[SSE] Error parsing message:', err)
          }
        }

        eventSource.onerror = (err) => {
          // Проверяем состояние соединения перед логированием ошибки
          if (eventSource.readyState === EventSource.CLOSED) {
            // Соединение закрыто - это нормально при переподключении
            setConnected(false)
            eventSource.close()
            eventSourceRef.current = null

            // Reconnect with exponential backoff
            reconnectAttemptsRef.current += 1
            const delay = Math.min(1000 * Math.pow(2, reconnectAttemptsRef.current), 30000)

            if (reconnectAttemptsRef.current <= 5) {
              console.log(`[SSE] Reconnecting in ${delay}ms (attempt ${reconnectAttemptsRef.current})`)
              setError(`Connection lost. Reconnecting in ${(delay / 1000).toFixed(0)}s...`)

              reconnectTimeoutRef.current = setTimeout(() => {
                connect()
              }, delay)
            } else {
              setError('Connection failed after multiple attempts')
              console.error('[SSE] Connection failed after multiple attempts')
            }
          } else {
            // Другие ошибки - логируем только если это не обычное переподключение
            if (eventSource.readyState !== EventSource.CONNECTING) {
              console.error('[SSE] Connection error:', err)
            }
          }
        }
      } catch (err) {
        console.error('[SSE] Error creating EventSource:', err)
        setError('Failed to establish connection')
      }
    }

    // Initial connection
    connect()

    // Cleanup
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close()
        eventSourceRef.current = null
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
    }
  }, [enabled])

  const reconnect = () => {
    reconnectAttemptsRef.current = 0
    setError(null)
    if (eventSourceRef.current) {
      eventSourceRef.current.close()
      eventSourceRef.current = null
    }
  }

  return {
    metrics,
    connected,
    error,
    reconnect,
  }
}
