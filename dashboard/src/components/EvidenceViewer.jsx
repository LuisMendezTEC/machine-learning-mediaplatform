import styles from './EvidenceViewer.module.css'

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
            return (
                <div className={styles.box}>
                    <div className={styles.imagePlaceholder}>
                        <span className={styles.icon}>🖼️</span>
                        Detección en imagen
                    </div>
                    {evidence.bounding_box && (
                        <code className={styles.code}>
                            Box: [X: {evidence.bounding_box.x}, Y: {evidence.bounding_box.y}, W: {evidence.bounding_box.w}, H: {evidence.bounding_box.h}]
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