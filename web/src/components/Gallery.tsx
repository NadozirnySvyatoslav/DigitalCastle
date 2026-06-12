import { useEffect, useState } from 'react'
import { api, Snapshot } from '../api'

export function Gallery() {
  const [snaps, setSnaps] = useState<Snapshot[]>([])
  const [zoom, setZoom] = useState<Snapshot | null>(null)
  const [err, setErr] = useState('')

  useEffect(() => {
    api.snapshots(200).then(setSnaps).catch((e) => setErr(e.message))
  }, [])

  if (err) return <div className="empty">Помилка: {err}</div>
  if (snaps.length === 0) return <div className="empty">Знімків ще немає</div>

  return (
    <div>
      <div className="grid">
        {snaps.map((s) => (
          <figure key={s.id} className="thumb" onClick={() => setZoom(s)}>
            <img src={s.url} alt={s.taken_at} loading="lazy" />
            <figcaption>
              {fmt(s.taken_at)}
              {s.trigger === 'motion' && <span className="tag motion">рух</span>}
              {s.trigger === 'manual' && <span className="tag">вручну</span>}
            </figcaption>
          </figure>
        ))}
      </div>

      {zoom && (
        <div className="lightbox" onClick={() => setZoom(null)}>
          <img src={zoom.url} alt={zoom.taken_at} />
          <div className="lightbox-cap">{fmt(zoom.taken_at)}</div>
        </div>
      )}
    </div>
  )
}

function fmt(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleString('uk-UA', { dateStyle: 'short', timeStyle: 'medium' })
}
