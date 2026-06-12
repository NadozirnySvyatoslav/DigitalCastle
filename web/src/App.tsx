import { useEffect, useState } from 'react'
import { api, Status } from './api'
import { LiveView } from './components/LiveView'
import { Gallery } from './components/Gallery'
import { Events } from './components/Events'
import { Settings } from './components/Settings'

type Tab = 'live' | 'snapshots' | 'events' | 'settings'

const TABS: Tab[] = ['live', 'snapshots', 'events', 'settings']

export default function App() {
  const [status, setStatus] = useState<Status | null>(null)
  const [tab, setTabState] = useState<Tab>(
    () => (TABS.includes(location.hash.slice(1) as Tab) ? (location.hash.slice(1) as Tab) : 'live'),
  )
  const setTab = (t: Tab) => {
    setTabState(t)
    location.hash = t
  }

  useEffect(() => {
    const load = () => api.status().then(setStatus).catch(() => setStatus(null))
    load()
    const t = setInterval(load, 10000)
    const onHash = () => {
      if (TABS.includes(location.hash.slice(1) as Tab)) setTabState(location.hash.slice(1) as Tab)
    }
    window.addEventListener('hashchange', onHash)
    return () => {
      clearInterval(t)
      window.removeEventListener('hashchange', onHash)
    }
  }, [])

  return (
    <div className="app">
      <header className="header">
        <div className="brand">
          <span className="logo">🎥</span>
          <div>
            <h1>NVR</h1>
            <span className="sub">{status?.model ?? 'камера…'}</span>
          </div>
        </div>
        <div className="status-pills">
          <span className={`pill ${status?.online ? 'ok' : 'bad'}`}>
            {status?.online ? '● онлайн' : '○ офлайн'}
          </span>
          <span className={`pill ${status?.motion_detection ? 'ok' : 'muted'}`}>
            рух: {status?.motion_detection ? 'увімк' : 'вимк'}
          </span>
          {status?.firmware && <span className="pill muted">{status.firmware}</span>}
        </div>
      </header>

      <nav className="tabs">
        <button className={tab === 'live' ? 'active' : ''} onClick={() => setTab('live')}>
          Перегляд
        </button>
        <button className={tab === 'snapshots' ? 'active' : ''} onClick={() => setTab('snapshots')}>
          Знімки
        </button>
        <button className={tab === 'events' ? 'active' : ''} onClick={() => setTab('events')}>
          Події
        </button>
        <button className={tab === 'settings' ? 'active' : ''} onClick={() => setTab('settings')}>
          Налаштування
        </button>
      </nav>

      <main className="main">
        {tab === 'live' && <LiveView caps={status?.capabilities} />}
        {tab === 'snapshots' && <Gallery />}
        {tab === 'events' && <Events />}
        {tab === 'settings' && <Settings />}
      </main>
    </div>
  )
}
