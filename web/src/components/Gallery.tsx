import { useEffect, useState } from 'react'
import { api, Snapshot } from '../api'
import { Pager } from './Pager'

const PAGE_SIZE = 24

export function Gallery() {
  const [snaps, setSnaps] = useState<Snapshot[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [zoom, setZoom] = useState<Snapshot | null>(null)
  const [err, setErr] = useState('')

  useEffect(() => {
    api
      .snapshots(PAGE_SIZE, page * PAGE_SIZE)
      .then((p) => {
        setSnaps(p.items)
        setTotal(p.total)
      })
      .catch((e) => setErr(e.message))
  }, [page])

  if (err) return <div className="empty">Помилка: {err}</div>
  if (total === 0) return <div className="empty">Знімків ще немає</div>

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

      <Pager page={page} pageSize={PAGE_SIZE} total={total} onChange={setPage} />

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
