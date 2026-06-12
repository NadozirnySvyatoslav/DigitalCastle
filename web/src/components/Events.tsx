import { useEffect, useState } from 'react'
import { api, EventItem } from '../api'

export function Events() {
  const [events, setEvents] = useState<EventItem[]>([])
  const [err, setErr] = useState('')

  useEffect(() => {
    api.events(200).then(setEvents).catch((e) => setErr(e.message))
  }, [])

  if (err) return <div className="empty">Помилка: {err}</div>
  if (events.length === 0) return <div className="empty">Подій ще немає</div>

  return (
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
  )
}

function fmt(iso: string): string {
  return new Date(iso).toLocaleString('uk-UA', { dateStyle: 'short', timeStyle: 'medium' })
}
