package web

import (
	"math"
	"strconv"

	"github.com/felicson/checkbot"
)

type ItemsList struct {
	Items         []*checkbot.User
	by            func(u1, u2 *checkbot.User) bool
	CurrentOffset int
	length        int
}

func (item ItemsList) Len() int { return len(item.Items) }

func (item ItemsList) Swap(i, j int) { item.Items[i], item.Items[j] = item.Items[j], item.Items[i] }

func (item ItemsList) Less(i, j int) bool {
	return item.by(item.Items[i], item.Items[j])
}

func (items *ItemsList) Pages() []int {

	var pages []int
	pages_num := math.Ceil(float64(items.length) / float64(DELIM))
	if pages_num > 0 {

		pages = make([]int, int(pages_num))
		for i := range pages {
			pages[i] = i + 1
		}
	}
	return pages

}
func (i *ItemsList) Offset(start string) (*ItemsList, error) {

	offset, err := strconv.Atoi(start)
	if err != nil {
		offset = 0
	}

	if offset > 0 {
		offset--
	}

	offset = offset * DELIM
	i.CurrentOffset = offset
	end := offset + DELIM
	length := len(i.Items)

	if length >= end {
		i.Items = i.Items[offset:end]
		return i, nil
	}

	if offset > length {
		return nil, ErrWrongOffset
	}
	i.Items = i.Items[offset:]
	return i, nil
}
