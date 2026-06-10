import styles from './EvidenceViewer.module.css'

const IMAGE_LABELS = {
    person: 'Persona Detectada',
    knife: 'Arma Blanca (Cuchillo)',
    gun: 'Arma de Fuego (Pistola / Fusil)',
    handgun: 'Arma de Fuego (Pistola)',
    pistol: 'Arma de Fuego (Pistola)',
    rifle: 'Arma de Fuego (Fusil)',
    firearm: 'Arma de Fuego',
    weapon: 'Arma / Peligro',
    explosion: 'Peligro de Explosión / Incendio',
    grenade: 'Granada / Artefacto Explosivo',
}

export default function EvidenceViewer({ workerType, evidence }) {
    if (!evidence) return <span className={styles.muted}>Sin evidencia registrada</span>

    switch (workerType) {
        case 'text':
            return (
                <div className={styles.box}>
                    <p className={styles.text}>"{evidence.text_fragment}"</p>
                    {evidence.keywords && (
                        <div className={styles.tags}>
                            {evidence.keywords.map(k => <span key={k} className={styles.tag}>{k}</span>)}
                        </div>
                    )}
                </div>
            )
        case 'image':
            const labelKey = evidence.label?.toLowerCase() || ''
            const translatedLabel = IMAGE_LABELS[labelKey] || `Objeto: ${evidence.label || 'Desconocido'}`
            return (
                <div className={styles.box}>
                    <div className={styles.imagePlaceholder}>
                        <span className={styles.icon}>🖼️</span>
                        {translatedLabel}
                    </div>
                    {evidence.bounding_box && (
                        <code className={styles.code}>
                            Área Detectada: [X: {evidence.bounding_box.x}, Y: {evidence.bounding_box.y}, Ancho: {evidence.bounding_box.w}px, Alto: {evidence.bounding_box.h}px]
                        </code>
                    )}
                </div>
            )
        case 'audio':
            return (
                <div className={styles.box}>
                    <span className={styles.timestamp}>⏱️ {evidence.timestamp || '00:00:00'}</span>
                    <p className={styles.text}>"{evidence.transcription}"</p>
                </div>
            )
        default:
            return <pre className={styles.code}>{JSON.stringify(evidence, null, 2)}</pre>
    }
}