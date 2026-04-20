import { useState, useEffect, useRef, useCallback } from 'react'

// In dev (npm run dev), Vite proxies /ws → ws://localhost:8080/ws
// In Docker (built image), nginx proxies /ws → coordinator:8080/ws
// Either way, we always connect to /ws on the same host.
const WS_URL = typeof import.meta !== 'undefined' && import.meta.env?.VITE_WS_URL
    ? import.meta.env.VITE_WS_URL
    : `ws://${window.location.host}/ws`

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

                // Normalize stats — coordinator sends a map of status→count,
                // so missing keys just default to 0.
                const rawStats = data.stats || {}
                const stats = {
                    pending: rawStats.pending ?? 0,
                    assigned: rawStats.assigned ?? 0,
                    running: rawStats.running ?? 0,
                    completed: rawStats.completed ?? 0,
                    failed: rawStats.failed ?? 0,
                }

                // queue_depth may be null if coordinator hasn't implemented it yet —
                // fall back to zeros so QueueDepth chart doesn't crash.
                const qd = data.queue_depth || {}
                const queue_depth = {
                    high: qd.high ?? 0,
                    normal: qd.normal ?? 0,
                    low: qd.low ?? 0,
                }

                setState({
                    workers: Array.isArray(data.workers) ? data.workers : [],
                    jobs: Array.isArray(data.jobs) ? data.jobs : [],
                    stats,
                    queue_depth,
                })
            } catch {
                // malformed message — ignore silently
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