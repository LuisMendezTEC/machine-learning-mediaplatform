import { useState, useEffect } from 'react'
import { api } from '../api'
import EvidenceViewer from './EvidenceViewer'
import styles from './CaseDetail.module.css'

const WORKER_NAMES = {
    text: 'Analizador de Texto',
    image: 'Analizador de Imagen',
    audio: 'Analizador de Audio',
}

const CATEGORY_NAMES = {
    sentiment: 'Tono Hostil / Negativo',
    keyword: 'Palabra Clave Detectada',
    weapon: 'Detección de Arma',
    violence: 'Violencia / Agresión',
    offensive: 'Lenguaje Ofensivo',
    threat: 'Amenaza',
    scam: 'Estafa / Fraude Financiero',
}

const RISK_NAMES = {
    low: 'Bajo',
    medium: 'Medio',
    high: 'Alto',
    critical: 'Crítico',
}

export default function CaseDetail({ caseId }) {
    const [report, setReport] = useState(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.getCaseReport(caseId)
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
                    <span className={styles.scoreValue}>{(report.case?.risk_score ?? 0).toFixed(1)} / 10</span>
                </div>
                <div className={styles.meta}>
                    <span>Generado: {new Date(report.generated_at).toLocaleString()}</span>
                    <span>Total Hallazgos: {report.findings_total || 0}</span>
                </div>
            </div>

            <div className={styles.grid}>
                {['text', 'image', 'audio'].map(type => {
                    const findings = report.findings_by_type[type] || []
                    if (findings.length === 0) return null

                    return (
                        <div key={type} className={styles.column}>
                            <h4 className={styles.typeTitle}>{WORKER_NAMES[type] || type.toUpperCase()}</h4>
                            <div className={styles.findingsList}>
                                {findings.map(f => {
                                    const job = report.jobs?.find(j => j.id === f.job_id)
                                    const fileName = job?.file_path ? job.file_path.split(/[/\\]/).pop() : ''
                                    return (
                                        <div key={f.id} className={styles.findingCard}>
                                            <div className={styles.findingHeader}>
                                                <span className={styles.category}>{CATEGORY_NAMES[f.category] || f.category}</span>
                                                <span className={`${styles.risk} ${styles[f.risk_level]}`}>{RISK_NAMES[f.risk_level] || f.risk_level}</span>
                                            </div>
                                            {fileName && (
                                                <div className={styles.fileName}>
                                                    📁 Archivo: <span className={styles.fileHighlight}>{fileName}</span>
                                                </div>
                                            )}
                                            <div className={styles.confidence}>Confianza: {(f.confidence * 100).toFixed(0)}%</div>
                                            <EvidenceViewer workerType={f.worker_type} evidence={f.evidence} />
                                        </div>
                                    )
                                })}
                            </div>
                        </div>
                    )
                })}
            </div>
        </div>
    )
}