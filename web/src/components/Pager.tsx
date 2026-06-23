interface PagerProps {
  page: number // 0-based
  pageSize: number
  total: number
  onChange: (page: number) => void
}

export function Pager({ page, pageSize, total, onChange }: PagerProps) {
  const pages = Math.max(1, Math.ceil(total / pageSize))
  if (pages <= 1) return null

  const from = page * pageSize + 1
  const to = Math.min(total, (page + 1) * pageSize)

  return (
    <nav className="pager">
      <button disabled={page === 0} onClick={() => onChange(0)} title="На початок">«</button>
      <button disabled={page === 0} onClick={() => onChange(page - 1)}>‹ Назад</button>
      <span className="pager-info">
        {from}–{to} з {total} · стор. {page + 1}/{pages}
      </span>
      <button disabled={page >= pages - 1} onClick={() => onChange(page + 1)}>Далі ›</button>
      <button disabled={page >= pages - 1} onClick={() => onChange(pages - 1)} title="В кінець">»</button>
    </nav>
  )
}
