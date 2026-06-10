import React, { useState } from 'react'
import { useSystemState } from '../hooks/useSystemState'
import CaseDetail from './CaseDetail'
import styles from './CaseList.module.css'

const STATUS_COLORS = {
    queued: { bg: '#1e293b', text: '#94a3b8' },
    processing: { bg: '#1e3a5f', text: '#60a5fa' },
    completed: { bg: '#14532d', text: '#86efac' },
    failed: { bg: '#450a0a', text: '#fca5a5' },
}

function StatusBadge({ status }) {
    const c = STATUS_COLORS[status] || { bg: '#1a1d2e', text: '#94a3b8' }
    return (
        <span className={styles.badge} style={{ background: c.bg, color: c.text }}>
            {status}
        </span>
    )
}

function RiskBadge({ score }) {
    if (score === undefined || score === null) return <span className={styles.muted}>—</span>
    let color = 'var(--green)'
    let label = 'Low'
    if (score >= 7.0) { color = 'var(--red)'; label = 'Critical' }
    else if (score >= 4.0) { color = 'var(--yellow)'; label = 'Medium' }
    
    return (
        <span className={styles.riskBadge} style={{ borderColor: color, color: color }}>
            {score.toFixed(1)} - {label}
        </span>
    )
}

export default function CaseList() {
    const { cases } = useSystemState()
    const [search, setSearch] = useState('')
    const [expandedId, setExpandedId] = useState(null)

    const visibleCases = cases
        .filter(c => !search || c.name.toLowerCase().includes(search.toLowerCase()) || c.id.includes(search))
        .sort((a, b) => new Date(b.created_at) - new Date(a.created_at))

    return (
        <div className={styles.wrap}>
            <div className={styles.toolbar}>
                <input
                    className={styles.search}
                    placeholder="Search Case by name or ID..."
                    value={search}
                    onChange={e => setSearch(e.target.value)}
                />
            </div>

            <div className={styles.tableWrap}>
                <table className={styles.table}>
                    <thead>
                        <tr>
                            <th>Case ID</th>
                            <th>Name</th>
                            <th>Description</th>
                            <th>Status</th>
                            <th>Priority</th>
                            <th>Risk Score</th>
                            <th>Created At</th>
                        </tr>
                    </thead>
                    <tbody>
                        {visibleCases.length === 0 && (
                            <tr>
                                <td colSpan={7} className={styles.empty}>No cases match the criteria or no cases loaded.</td>
                            </tr>
                        )}
                        {visibleCases.map(c => (
                            <React.Fragment key={c.id}>
                                <tr 
                                    className={`${styles.row} ${expandedId === c.id ? styles.rowExpanded : ''}`}
                                    onClick={() => setExpandedId(expandedId === c.id ? null : c.id)}
                                >
                                    <td className={styles.caseId}>{c.id.slice(0, 8)}…</td>
                                    <td className={styles.name}>{c.name}</td>
                                    <td className={styles.muted}>{c.description || '—'}</td>
                                    <td><StatusBadge status={c.status} /></td>
                                    <td className={styles.muted}>{c.priority}</td>
                                    <td><RiskBadge score={c.risk_score} /></td>
                                    <td className={styles.muted}>{new Date(c.created_at).toLocaleString()}</td>
                                </tr>
                                {expandedId === c.id && (
                                    <tr>
                                        <td colSpan={7} style={{ padding: 0 }}>
                                            <CaseDetail caseId={c.id} />
                                        </td>
                                    </tr>
                                )}
                            </React.Fragment>
                        ))}
                    </tbody>
                </table>
            </div>
            <div className={styles.count}>{visibleCases.length} of {cases.length} cases</div>
        </div>
    )
}