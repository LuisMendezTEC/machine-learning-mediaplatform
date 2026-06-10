import { useState, useEffect } from 'react'
import EvidenceViewer from './EvidenceViewer'
import styles from './CaseDetail.module.css'

export default function CaseDetail({ caseId }) {
    const [report, setReport] = useState(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        // En un entorno real esto usaría api.js, pero hacemos un fetch directo al coordinator
        fetch(`http://localhost:8080/cases/${caseId}/report`)
            .then(res => res.json())
            .then(data => {
                setReport(data)
                setLoading(false)
            })
            .catch(() => {
                setLoading(false)
            })
    }, [caseId])

    if (loading) return <div className={styles.loading}>Cargando reporte consolidado...</div>
    if (!report || !report.findings_by_type) return <div className={styles.empty}>No hay hallazgos para este caso o el motor de reportes no ha finalizado.</div>

    return (
        <div className={styles.detailWrap}>
            <div className={styles.header}>
                <div className={styles.scoreCard}>
                    <span className={styles.scoreLabel}>Riesgo Final</span>
                    <span className={styles.scoreValue}>{report.risk_score?.toFixed(1)} / 10</span>
                </div>
                <div className={styles.meta}>
                    <span>Generado: {new Date(report.generated_at).toLocaleString()}</span>
                    <span>Total Hallazgos: {report.timeline?.length || 0}</span>
                </div>
            </div>

            <div className={styles.grid}>
                {['text', 'image', 'audio'].map(type => {
                    const findings = report.findings_by_type[type] || []
                    if (findings.length === 0) return null

                    return (
                        <div key={type} className={styles.column}>
                            <h4 className={styles.typeTitle}>{type.toUpperCase()} WORKER</h4>
                            <div className={styles.findingsList}>
                                {findings.map(f => (
                                    <div key={f.id} className={styles.findingCard}>
                                        <div className={styles.findingHeader}>
                                            <span className={styles.category}>{f.category}</span>
                                            <span className={`${styles.risk} ${styles[f.risk_level]}`}>{f.risk_level}</span>
                                        </div>
                                        <div className={styles.confidence}>Confianza: {(f.confidence * 100).toFixed(0)}%</div>
                                        <EvidenceViewer workerType={f.worker_type} evidence={f.evidence} />
                                    </div>
                                ))}
                            </div>
                        </div>
                    )
                })}
            </div>
        </div>
    )
}