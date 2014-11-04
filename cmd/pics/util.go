package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/rwcarlsen/gallery/piclib"
)

func EscapeNotes(s string) string {
	data, err := json.Marshal(strings.TrimSpace(s))
	if err != nil {
		panic(err)
	}
	return string(data[1 : len(data)-1])
}

func WriteLines(w io.Writer, pics ...*piclib.Pic) error {
	minwidth := 8
	tabwidth := 4
	pad := 2
	tw := tabwriter.NewWriter(w, minwidth, tabwidth, pad, ' ', 0)
	defer tw.Flush()

	for _, p := range pics {
		notes, err := p.GetNotes()
		if err != nil {
			return err
		}
		notes = EscapeNotes(notes)

		// truncate long pic name
		nm := p.Name
		if len(p.Name) > 22 {
			nm = "..." + p.Name[len(p.Name)-22:]
		}

		tm := p.Taken
		fmt.Fprintf(tw, "%v\t%v\t%v\t\"%v\"\t%v\n", p.Id, tm.Unix(), tm.Format("2006/1/2"), nm, notes)
	}
	return nil
}

func ParseLines(r io.Reader) (picids []int, err error) {
	buf := bufio.NewReader(r)

	for {
		s, err := buf.ReadString('\n')
		fields := strings.Split(strings.TrimSpace(s), " ")
		if len(fields) > 0 {
			v := strings.TrimSpace(fields[0])
			if len(v) > 0 {
				id, err := strconv.Atoi(strings.TrimSpace(fields[0]))
				if err != nil {
					return nil, err
				}
				picids = append(picids, id)
			}
		}

		if err != nil {
			break
		}
	}

	if err != io.EOF && err != nil {
		return nil, err
	}
	return picids, nil
}

func idsOrStdin(args []string) []*piclib.Pic {
	var err error
	var picids []int
	if len(args) == 0 {
		picids, err = ParseLines(os.Stdin)
		check(err)
	} else {
		for _, idstr := range args {
			id, err := strconv.Atoi(idstr)
			check(err)
			picids = append(picids, id)
		}
	}

	var pics []*piclib.Pic
	for _, id := range picids {
		p, err := lib.Open(id)
		check(err)
		pics = append(pics, p)
	}
	return pics
}

func check(err error) {
	if err != nil {
		log.Fatal("[ERROR] ", err)
	}
}
