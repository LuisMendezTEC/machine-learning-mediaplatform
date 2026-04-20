import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts'
import styles from './QueueDepth.module.css'

const COLORS = { high: '#ef4444', normal: '#6366f1', low: '#22c55e' }

export default function QueueDepth({ queue_depth }) {
    const data = [
        { name: 'HIGH', value: queue_depth?.high ?? 0 },
        { name: 'NORMAL', value: queue_depth?.normal ?? 0 },
        { name: 'LOW', value: queue_depth?.low ?? 0 },
    ]

    const total = data.reduce((s, d) => s + d.value, 0)

    return (
        <div className={styles.wrap}>
            <div className={styles.header}>
                <span className={styles.title}>Queue Depth</span>
                <span className={styles.total}>{total} pending</span>
            </div>
            <ResponsiveContainer width="100%" height={100}>
                <BarChart data={data} margin={{ top: 4, right: 4, bottom: 0, left: -20 }}>
                    <XAxis dataKey="name" tick={{ fill: '#64748b', fontSize: 11 }} axisLine={false} tickLine={false} />
                    <YAxis tick={{ fill: '#64748b', fontSize: 11 }} axisLine={false} tickLine={false} allowDecimals={false} />
                    <Tooltip
                        contentStyle={{ background: '#1a1d2e', border: '1px solid #2a2d3e', borderRadius: 6, fontSize: 12 }}
                        cursor={{ fill: 'rgba(255,255,255,0.04)' }}
                    />
                    <Bar dataKey="value" radius={[4, 4, 0, 0]}>
                        {data.map((entry) => (
                            <Cell key={entry.name} fill={COLORS[entry.name.toLowerCase()]} />
                        ))}
                    </Bar>
                </BarChart>
            </ResponsiveContainer>
        </div>
    )
}