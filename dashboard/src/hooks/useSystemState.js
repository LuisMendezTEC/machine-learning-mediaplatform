import { useState, useEffect, useRef, useCallback } from 'react'

const WS_URL = typeof import.meta !== 'undefined' && import.meta.env?.VITE_WS_URL
    ? import.meta.env.VITE_WS_URL
    : `ws://${window.location.host}/ws`

const EMPTY_STATE = {
    workers: [],
    jobs: [],
    cases: [], // <- Agregado para soportar los casos
    stats: { pending: 0, assigned: 0, running: 0, completed: 0, failed: 0 },
    queue_depth: { high: 0, normal: 0, low: 0 },
}

export function useSystemState() {
    const [state, setState] = useState(EMPTY_STATE)
    const [connected, setConnected] = useState(false)
    const wsRef = useRef(null)
    const retryRef = useRef(null)
    const refreshTimeRef = useRef(null)

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

                const allJobs = Array.isArray(data.jobs) ? data.jobs : []
                let filteredJobs = allJobs

                if (refreshTimeRef.current) {
                    filteredJobs = allJobs.filter(job => {
                        const jobCreatedAt = new Date(job.created_at).getTime()
                        return jobCreatedAt > refreshTimeRef.current
                    })
                }

                const stats = {
                    pending: filteredJobs.filter(j => j.status === 'pending').length,
                    assigned: filteredJobs.filter(j => j.status === 'assigned').length,
                    running: filteredJobs.filter(j => j.status === 'running').length,
                    completed: filteredJobs.filter(j => j.status === 'completed').length,
                    failed: filteredJobs.filter(j => j.status === 'failed').length,
                }

                let queue_depth = {}
                if (refreshTimeRef.current) {
                    queue_depth = { high: 0, normal: 0, low: 0 }
                } else {
                    const qd = data.queue_depth || {}
                    queue_depth = {
                        high: qd.high ?? 0,
                        normal: qd.normal ?? 0,
                        low: qd.low ?? 0,
                    }
                }

                const liveJobs = filteredJobs.filter(job => 
                    job.status === 'pending' || job.status === 'assigned' || job.status === 'running'
                )

                // Extraer los casos enviados por el coordinator
                const liveCases = Array.isArray(data.cases) ? data.cases : []

                setState({
                    workers: Array.isArray(data.workers) ? data.workers : [],
                    jobs: liveJobs,
                    cases: liveCases, // <- Estado actualizado
                    stats,
                    queue_depth,
                })
            } catch {
                // ignorar errores de parseo
            }
        }

        ws.onclose = () => {
            setConnected(false)
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

    const refresh = useCallback(async () => {
        refreshTimeRef.current = Date.now()
        setState(prev => ({
            ...prev,
            jobs: [],
            cases: [], // Limpiar visualmente hasta que lleguen los nuevos del WS
            stats: { pending: 0, assigned: 0, running: 0, completed: 0, failed: 0 },
            queue_depth: { high: 0, normal: 0, low: 0 },
        }))
    }, [])

    return { ...state, connected, refresh }
}