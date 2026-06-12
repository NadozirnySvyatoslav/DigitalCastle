import { useEffect, useState } from 'react'
import { api, AppConfig } from '../api'

export function Settings() {
  const [cfg, setCfg] = useState<AppConfig | null>(null)
  const [chatsText, setChatsText] = useState('')
  const [msg, setMsg] = useState('')
  const [saving, setSaving] = useState(false)
  const [restarting, setRestarting] = useState(false)

  useEffect(() => {
    api.getConfig().then((c) => {
      setCfg(c)
      setChatsText((c.telegram.allowed_chats ?? []).join(', '))
    }).catch((e) => setMsg('❌ ' + e.message))
  }, [])

  if (!cfg) return <div className="empty">{msg || 'Завантаження…'}</div>

  // оновлення вкладеного поля
  const set = (section: keyof AppConfig, key: string, value: unknown) =>
    setCfg({ ...cfg, [section]: { ...cfg[section], [key]: value } })

  const parseChats = (): number[] =>
    chatsText
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
      .map(Number)
      .filter((n) => !Number.isNaN(n))

  const save = async (thenRestart: boolean) => {
    setSaving(true)
    setMsg('')
    try {
      const toSave: AppConfig = {
        ...cfg,
        telegram: { ...cfg.telegram, allowed_chats: parseChats() },
      }
      await api.saveConfig(toSave)
      if (thenRestart) {
        setRestarting(true)
        await api.restart()
        // чекаємо підняття сервісу і перезавантажуємо сторінку
        setTimeout(() => waitAndReload(), 1500)
      } else {
        setMsg('💾 Збережено. Натисни «Застосувати», щоб перезапустити сервіс.')
      }
    } catch (e) {
      setMsg('❌ ' + (e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="settings">
      {restarting && (
        <div className="restart-overlay">
          <div className="spinner" />
          <p>Перезапуск сервісу…</p>
        </div>
      )}

      <Section title="📷 Камера">
        <Field label="Адреса (host)" v={cfg.camera.host} onChange={(x) => set('camera', 'host', x)} />
        <Field label="Логін" v={cfg.camera.username} onChange={(x) => set('camera', 'username', x)} />
        <Field label="Пароль" type="password" v={cfg.camera.password} onChange={(x) => set('camera', 'password', x)} />
        <Field label="RTSP головний" v={cfg.camera.rtsp_main} onChange={(x) => set('camera', 'rtsp_main', x)} wide />
        <Field label="RTSP додатковий" v={cfg.camera.rtsp_sub} onChange={(x) => set('camera', 'rtsp_sub', x)} wide />
      </Section>

      <Section title="🤖 Telegram">
        <Field label="Токен бота" type="password" v={cfg.telegram.token} onChange={(x) => set('telegram', 'token', x)} wide />
        <Field label="Основний chat_id" type="number" v={String(cfg.telegram.chat_id)} onChange={(x) => set('telegram', 'chat_id', Number(x))} />
        <div className="field wide">
          <label>Дозволені чати (через кому)</label>
          <input value={chatsText} onChange={(e) => setChatsText(e.target.value)} placeholder="-5293721438, ..." />
        </div>
        <Toggle label="Сповіщення про рух у Telegram" v={cfg.telegram.notify_on_motion} onChange={(x) => set('telegram', 'notify_on_motion', x)} />
      </Section>

      <Section title="🎞️ Запис">
        <Field label="Інтервал знімків (напр. 5m, 30s)" v={cfg.capture.snapshot_interval} onChange={(x) => set('capture', 'snapshot_interval', x)} />
        <Field label="Тривалість кліпу на рух (с)" type="number" v={String(cfg.capture.motion_clip_seconds)} onChange={(x) => set('capture', 'motion_clip_seconds', Number(x))} />
        <Field label="Тека даних" v={cfg.capture.data_dir} onChange={(x) => set('capture', 'data_dir', x)} />
      </Section>

      <Section title="🧹 Зберігання (автоочистка)">
        <Field label="Макс. вік, днів (0 = ∞)" type="number" v={String(cfg.retention.max_days)} onChange={(x) => set('retention', 'max_days', Number(x))} />
        <Field label="Макс. розмір, ГБ (0 = ∞)" type="number" v={String(cfg.retention.max_size_gb)} onChange={(x) => set('retention', 'max_size_gb', Number(x))} />
        <Field label="Інтервал прибирання (напр. 1h)" v={cfg.retention.interval} onChange={(x) => set('retention', 'interval', x)} />
      </Section>

      <Section title="🌐 Сервер">
        <Field label="Адреса (напр. :8080)" v={cfg.server.addr} onChange={(x) => set('server', 'addr', x)} />
      </Section>

      <div className="settings-actions">
        <button className="btn" onClick={() => save(false)} disabled={saving || restarting}>💾 Зберегти</button>
        <button className="btn primary" onClick={() => save(true)} disabled={saving || restarting}>✅ Зберегти і застосувати (перезапуск)</button>
        {msg && <span className="settings-msg">{msg}</span>}
      </div>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <fieldset className="settings-section">
      <legend>{title}</legend>
      <div className="settings-grid">{children}</div>
    </fieldset>
  )
}

function Field({ label, v, onChange, type = 'text', wide = false }: {
  label: string; v: string; onChange: (v: string) => void; type?: string; wide?: boolean
}) {
  return (
    <div className={'field' + (wide ? ' wide' : '')}>
      <label>{label}</label>
      <input type={type} value={v} onChange={(e) => onChange(e.target.value)} />
    </div>
  )
}

function Toggle({ label, v, onChange }: { label: string; v: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="toggle field wide">
      <input type="checkbox" checked={v} onChange={(e) => onChange(e.target.checked)} />
      <span>{label}</span>
    </label>
  )
}

// waitAndReload пінгує API, доки сервіс не підніметься, потім перезавантажує сторінку.
function waitAndReload(attempt = 0) {
  fetch('/api/status')
    .then((r) => {
      if (r.ok) window.location.reload()
      else throw new Error('not ready')
    })
    .catch(() => {
      if (attempt < 20) setTimeout(() => waitAndReload(attempt + 1), 1000)
      else window.location.reload()
    })
}
