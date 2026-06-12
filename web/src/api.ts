export interface Capabilities {
  driver: string
  lens: boolean
  flip: boolean
  hardware_motion: boolean
  motion_config: boolean
  time_sync: boolean
}

export interface Status {
  model: string
  firmware: string
  online: boolean
  motion_detection: boolean
  capabilities: Capabilities
}

export interface Snapshot {
  id: number
  path: string
  taken_at: string
  size: number
  trigger: string
  url: string
}

export interface EventItem {
  id: number
  type: string
  path: string
  note: string
  created_at: string
  url: string
}

export interface AppConfig {
  camera: { host: string; username: string; password: string; rtsp_main: string; rtsp_sub: string }
  telegram: { token: string; chat_id: number; allowed_chats: number[]; notify_on_motion: boolean }
  capture: { snapshot_interval: string; data_dir: string; motion_clip_seconds: number }
  retention: { max_days: number; max_size_gb: number; interval: string }
  server: { addr: string }
}

async function getJSON<T>(url: string): Promise<T> {
  const r = await fetch(url)
  if (!r.ok) throw new Error(`${url}: HTTP ${r.status}`)
  return r.json() as Promise<T>
}

export const api = {
  status: () => getJSON<Status>('/api/status'),
  snapshots: (limit = 100) => getJSON<Snapshot[]>(`/api/snapshots?limit=${limit}`),
  events: (limit = 100) => getJSON<EventItem[]>(`/api/events?limit=${limit}`),
  capture: () => fetch('/api/snapshot', { method: 'POST' }).then((r) => r.json()),
  clip: () => fetch('/api/clip', { method: 'POST' }).then((r) => r.json()),
  lens: (action: 'zoom_in' | 'zoom_out' | 'focus_near' | 'focus_far') =>
    fetch(`/api/lens/${action}`, { method: 'POST' }).then((r) => {
      if (!r.ok) throw new Error('lens')
      return r.json()
    }),
  flip: () =>
    fetch('/api/flip', { method: 'POST' }).then((r) => {
      if (!r.ok) throw new Error('flip')
      return r.json()
    }),
  getConfig: () => getJSON<AppConfig>('/api/config'),
  saveConfig: async (cfg: AppConfig) => {
    const r = await fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    })
    if (!r.ok) throw new Error(await r.text())
    return r.json()
  },
  restart: () => fetch('/api/restart', { method: 'POST' }).then((r) => r.json()),
  liveURL: () => `/api/snapshot/live?t=${Date.now()}`,
}
