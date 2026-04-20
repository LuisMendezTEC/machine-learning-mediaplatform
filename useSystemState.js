import { useState, useEffect, useRef, useCallback } from 'react'

const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8080/ws'

// Initial empty state so the UI never crashes before first message
const EMPTY_STATE = {
    workers: [],
    jobs: [],
    stats: { pending: 0, assigned: 0, running: 0, completed: 0, failed: 0 },
    queue_depth: { high: 0, normal: 0, low: 0 },
}

export function useSystemState() {
    const [state, setState] = useState(EMPTY_STATE)
    const [connected, setConnected] = useState(false)
    const wsRef = useRef(null)
    const retryRef = useRef(null)

    const connect = useCallback(() => {
        // Don't double-connect
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) return

        const ws = new WebSocket(WS_URL)
        wsRef.current = ws

        ws.onopen = () => {
            setConnected(true)
            if (retryRef.current) {
                clearTimeout(retryRef.current)
                retryRef.current = null
            }
        }

        ws.onmessage = (e) => {
            try {
                const data = JSON.parse(e.data)
                setState({
                    workers: data.workers || [],
                    jobs: data.jobs || [],
                    stats: data.stats || EMPTY_STATE.stats,
                    queue_depth: data.queue_depth || EMPTY_STATE.queue_depth,
                })
            } catch {
                // malformed message — ignore
            }
        }

        ws.onclose = () => {
            setConnected(false)
            // Reconnect after 3 seconds
            retryRef.current = setTimeout(connect, 3000)
        }

        ws.onerror = () => {
            ws.close()
        }
    }, [])

    useEffect(() => {
        connect()
        return () => {
            if (retryRef.current) clearTimeout(retryRef.current)
            if (wsRef.current) wsRef.current.close()
        }
    }, [connect])

    return { ...state, connected }
}