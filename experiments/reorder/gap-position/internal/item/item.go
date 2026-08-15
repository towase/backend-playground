package item

import "errors"

var (
	ErrItemNotFound = errors.New("item not found")
	ErrPreviousSame = errors.New("id and previousId must be different")
	ErrNoGap        = errors.New("no gap between positions")
)

type Item struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int64  `json:"position"`
}
