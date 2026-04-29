package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// Cursor encodes the position of the last item returned. We sort by
// (created_at DESC, id DESC), so the next page starts at any row strictly
// older than this tuple.
type Cursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

func (c Cursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(b)
}

func Decode(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, errors.New("empty cursor")
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, err
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return Cursor{}, err
	}
	return c, nil
}
