import { useEffect, useRef, useState } from 'react'
import { api, Capabilities } from '../api'

export function LiveView({ caps }: { caps?: Capabilities }) {
  const [src, setSrc] = useState(api.liveURL())
  const [busy, setBusy] = useState('')
  const [msg, setMsg] = useState('')
  const [online, setOnline] = useState(true)
  const timer = useRef<number>()

  useEffect(() => {
    // оновлюємо кадр раз на секунду (live через знімки)
    timer.current = window.setInterval(() => setSrc(api.liveURL()), 1000)
    return () => clearInterval(timer.current)
  }, [])

  const flash = (t: string) => {
    setMsg(t)
    window.setTimeout(() => setMsg(''), 4000)
  }

  const snap = async () => {
    setBusy('snap')
    try {
      await api.capture()
      flash('📸 Знімок збережено')
    } catch (e) {
      flash('❌ ' + (e as Error).message)
    } finally {
      setBusy('')
    }
  }

  const clip = async () => {
    setBusy('clip')
    try {
      await api.clip()
      flash('🎬 Кліп записано — дивись у «Події»')
    } catch (e) {
      flash('❌ ' + (e as Error).message)
    } finally {
      setBusy('')
    }
  }

  const lens = async (action: 'zoom_in' | 'zoom_out' | 'focus_near' | 'focus_far', label: string) => {
    setBusy(action)
    try {
      await api.lens(action)
      flash(label)
      // оновити кадр після руху об'єктива
      window.setTimeout(() => setSrc(api.liveURL()), 1500)
    } catch {
      flash('❌ помилка керування об’єктивом')
    } finally {
      setBusy('')
    }
  }

  const flip = async () => {
    setBusy('flip')
    try {
      const r = await api.flip()
      flash('🔄 Переворот: ' + (r.flip ? 'увімк' : 'вимк'))
    } catch {
      flash('❌ помилка перевороту')
    } finally {
      setBusy('')
    }
  }

  return (
    <div className="live">
      <div className="live-frame">
        <img
          src={src}
          alt="live"
          className={online ? '' : 'hidden'}
          onLoad={() => setOnline(true)}
          onError={() => setOnline(false)}
        />
        {online ? (
          <span className="live-badge">● LIVE</span>
        ) : (
          <div className="live-offline">
            <span className="live-offline-icon">📵</span>
            <p>Камера недоступна</p>
            <small>Очікую підключення…</small>
          </div>
        )}
      </div>

      <div className="controls">
        <button className="btn primary" onClick={snap} disabled={!!busy}>
          {busy === 'snap' ? '…' : '📸 Знімок'}
        </button>
        <button className="btn" onClick={clip} disabled={!!busy}>
          {busy === 'clip' ? 'запис…' : '🎬 Кліп'}
        </button>

        {(caps?.lens || caps?.flip) && (
          <div className="lens-group">
            <span className="lens-label">Об'єктив:</span>
            {caps?.lens && <>
              <button className="btn ghost" onClick={() => lens('zoom_out', '🔍 Зум −')} disabled={!!busy}>🔍−</button>
              <button className="btn ghost" onClick={() => lens('zoom_in', '🔍 Зум +')} disabled={!!busy}>🔍+</button>
              <button className="btn ghost" onClick={() => lens('focus_far', '🎯 Фокус далі')} disabled={!!busy}>фокус −</button>
              <button className="btn ghost" onClick={() => lens('focus_near', '🎯 Фокус ближче')} disabled={!!busy}>фокус +</button>
            </>}
            {caps?.flip && <button className="btn ghost" onClick={flip} disabled={!!busy}>🔄 flip</button>}
          </div>
        )}
      </div>

      {msg && <div className="toast">{msg}</div>}
    </div>
  )
}
