
package main

import (
	"github.com/rwcarlsen/gallery/piclib"
)

type year struct {
	Year      int
	StartPage int
	Months    []*month
}

func (y *year) reverseMonths() {
	end := len(y.Months) - 1
	for i := 0; i < len(y.Months)/2; i++ {
		y.Months[i], y.Months[end-i] = y.Months[end-i], y.Months[i]
	}
}

type month struct {
	Name string
	Page int
}

type thumbData struct {
	Path  string
	Date  string
	Index int
}

type newFirst []*piclib.Photo

func (pl newFirst) Less(i, j int) bool {
	itm := pl[i].Taken
	jtm := pl[j].Taken
	return itm.After(jtm)
}

func (pl newFirst) Len() int {
	return len(pl)
}

func (pl newFirst) Swap(i, j int) {
	pl[i], pl[j] = pl[j], pl[i]
}

