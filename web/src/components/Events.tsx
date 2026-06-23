import { useEffect, useState } from 'react'
import { api, EventItem } from '../api'
import { Pager } from './Pager'

const PAGE_SIZE = 20

export function Events() {
  const [events, setEvents] = useState<EventItem[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [err, setErr] = useState('')

  useEffect(() => {
    api
      .events(PAGE_SIZE, page * PAGE_SIZE)
      .then((p) => {
        setEvents(p.items)
        setTotal(p.total)
      })
      .catch((e) => setErr(e.message))
  }, [page])

  if (err) return <div className="empty">Помилка: {err}</div>
  if (total === 0) return <div className="empty">Подій ще немає</div>

  return (
    <div>
      <ul className="events">
        {events.map((e) => (
          <li key={e.id} className="event">
            <div className="event-head">
              <span className={`tag ${e.type === 'motion' ? 'motion' : 'rec'}`}>
                {e.type === 'motion' ? '🏃 рух' : '🎥 запис'}
              </span>
              <time>{fmt(e.created_at)}</time>
            </div>
            {e.type === 'recording' && e.url && (
              <video controls preload="none" src={e.url} className="event-video" />
            )}
            {e.note && <p className="event-note">{e.note}</p>}
          </li>
        ))}
      </ul>

      <Pager page={page} pageSize={PAGE_SIZE} total={total} onChange={setPage} />
    </div>
  )
}

function fmt(iso: string): string {
  return new Date(iso).toLocaleString('uk-UA', { dateStyle: 'short', timeStyle: 'medium' })
}
