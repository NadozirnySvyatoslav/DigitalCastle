package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SearchQuery — структурований результат розбору запиту користувача.
type SearchQuery struct {
	Kind  string `json:"kind"`  // "recording" | "snapshot"
	From  string `json:"from"`  // ISO 8601 або ""
	To    string `json:"to"`    // ISO 8601 або ""
	Reply string `json:"reply"` // дружній текст-відповідь українською
	Limit int    `json:"limit"` // скільки показати (1..10)
}

const searchSystemPrompt = `Ти — асистент відеореєстратора (NVR) з IP-камерою.
Користувач пише природною мовою, що хоче знайти. Твоя задача — перетворити це
на структурований JSON-фільтр для пошуку у базі знімків та відеозаписів.

Відповідай ВИКЛЮЧНО JSON-об'єктом такої форми:
{
  "kind": "recording" | "snapshot",
  "from": "<ISO 8601 з таймзоною або порожньо>",
  "to":   "<ISO 8601 з таймзоною або порожньо>",
  "limit": <число 1..10>,
  "reply": "<коротка дружня відповідь українською, що саме шукаєш>"
}

Правила:
- "записи", "відео", "кліпи", "рух" → kind="recording".
- "знімки", "фото", "фотографії", "картинки" → kind="snapshot".
- Якщо незрозуміло — kind="recording".
- Відносні дати ("вчора", "сьогодні", "останні 2 години", "вранці", "на вихідних")
  перетворюй на абсолютні from/to у таймзоні поточного часу.
- "вранці"=06:00-12:00, "вдень"=12:00-18:00, "ввечері"=18:00-23:00, "вночі"=00:00-06:00.
- Якщо період не вказано — лиши from/to порожніми (шукатимемо останні).
- limit за замовчуванням 10.
- reply — короткий, напр. "Знайшов записи за вчора з 14:00 до 18:00" (без вигадування кількості).`

// ParseSearch розбирає запит користувача у SearchQuery, враховуючи поточний час now.
func (c *Client) ParseSearch(ctx context.Context, query string, now time.Time) (*SearchQuery, error) {
	user := fmt.Sprintf("Поточний час: %s\nЗапит: %s", now.Format(time.RFC3339), query)
	raw, err := c.ChatJSON(ctx, searchSystemPrompt, user)
	if err != nil {
		return nil, err
	}
	var q SearchQuery
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &q); err != nil {
		return nil, fmt.Errorf("розбір відповіді LLM: %w (%s)", err, raw)
	}
	if q.Kind != "snapshot" {
		q.Kind = "recording"
	}
	if q.Limit < 1 || q.Limit > 10 {
		q.Limit = 10
	}
	return &q, nil
}

// TimeRange повертає розібрані from/to як time.Time (нульові, якщо порожні).
func (q *SearchQuery) TimeRange() (from, to time.Time) {
	from, _ = parseTime(q.From)
	to, _ = parseTime(q.To)
	return
}

func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
