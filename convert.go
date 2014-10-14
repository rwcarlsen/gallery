package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/rwcarlsen/gallery/piclib"
)

var libpath = flag.String("lib", piclib.DefaultPath(), "path to picture library")

func main() {
	flag.Parse()
	lib, err := piclib.Open(*libpath)
	if err != nil {
		log.Fatal(lib)
	}

	r := bufio.NewReader(os.Stdin)

	i := 0
	for {
		i++
		line, buferr := r.ReadString('\n')
		path := strings.TrimSpace(line)
		fmt.Printf("adding %v\n", path)

		pic, err := lib.AddFile(path)
		if err != nil {
			log.Fatal(err)
		}

		notes, _, err := Notes(path)
		if err != nil {
			log.Fatal(err)
		}

		err = pic.SetNotes(notes)
		if err != nil {
			log.Fatal(err)
		}

		if buferr != nil {
			break
		}
	}
	fmt.Printf("added %v pics to library\n", i)
}

type Meta struct{}

func Notes(path string) (notes string, m *Meta, err error) {
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return "", &Meta{}, nil
	} else if err != nil {
		return "", nil, err
	}

	notes = string(data)

	buf := bytes.NewBuffer(data)
	dec := json.NewDecoder(buf)
	m = &Meta{}
	if err := dec.Decode(&m); err == nil {
		data, err := ioutil.ReadAll(dec.Buffered())
		if err != nil {
			return "", nil, err
		}
		notes = string(data)
	} else {
		m = nil
	}

	return notes, m, nil
}
