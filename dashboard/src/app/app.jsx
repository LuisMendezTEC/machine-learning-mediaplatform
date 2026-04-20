import { useSystemState } from '../hooks/useSystemState'
import WorkerCard from '../components/WorkerCard'
import JobTable from '../components/JobTable'
import StatsBar from '../components/StatsBar'
import QueueDepth from '../components/QueueDepth'
import styles from './app.module.css'

function ConnectionBadge({ connected }) {
    return (
        <div className={`${styles.connBadge} ${connected ? styles.connOk : styles.connErr}`}>
            <span className={styles.connDot} />
            {connected ? 'Live' : 'Reconnecting…'}
        </div>
    )
}

export default function App() {
    const { workers, jobs, stats, queue_depth, connected } = useSystemState()

    return (
        <div className={styles.layout}>
            {/* ── Header ── */}
            <header className={styles.header}>
                <div className={styles.headerLeft}>
                    <span className={styles.logo}>⚡ MediaPlatform</span>
                    <span className={styles.subtitle}>Distributed Processing Dashboard</span>
                </div>
                <ConnectionBadge connected={connected} />
            </header>

            <main className={styles.main}>
                {/* ── Stats row ── */}
                <section className={styles.section}>
                    <StatsBar stats={stats} />
                </section>

                {/* ── Workers + Queue ── */}
                <section className={styles.section}>
                    <div className={styles.sectionHeader}>
                        <h2 className={styles.sectionTitle}>Worker Nodes</h2>
                        <span className={styles.sectionCount}>{workers.length} registered</span>
                    </div>
                    <div className={styles.workerGrid}>
                        {workers.length === 0 && (
                            <p className={styles.empty}>No workers registered yet. Run <code>make up</code> to start workers.</p>
                        )}
                        {workers.map(w => <WorkerCard key={w.id} worker={w} />)}
                        <QueueDepth queue_depth={queue_depth} />
                    </div>
                </section>

                {/* ── Job table ── */}
                <section className={styles.section}>
                    <div className={styles.sectionHeader}>
                        <h2 className={styles.sectionTitle}>Jobs</h2>
                        <span className={styles.sectionCount}>{jobs.length} total</span>
                    </div>
                    <JobTable jobs={jobs} />
                </section>
            </main>
        </div>
    )
}