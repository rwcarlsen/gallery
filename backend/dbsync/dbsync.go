package dbsync

import (
	"errors"
	"fmt"

	"github.com/rwcarlsen/gallery/piclib"
)

const (
	Cdry = 1 << iota
)

const infinity = 500000

func OneWay(path string, config int, from, to piclib.Backend) (results []string, err error) {
	names, err := from.ListN(path, infinity)
	if err != nil {
		return nil, err
	}

	fromObj := make(map[string]bool)
	for _, name := range names {
		fromObj[name] = true
	}

	names, err = to.ListN(path, infinity)
	if err != nil {
		return nil, err
	}

	toObj := make(map[string]bool)
	for _, name := range names {
		toObj[name] = true
	}

	errs := false
	for objName, _ := range fromObj {
		if !toObj[objName] {
			results = append(results, fmt.Sprintf("sync from %v to %v: %v", from.Name(), to.Name(), objName))
			if config&Cdry != 0 {
				continue
			}
			data, err := from.Get(objName)
			if err != nil {
				results = append(results, err.Error())
				errs = true
				continue
			}
			if err := to.Put(objName, data); err != nil {
				results = append(results, err.Error())
				errs = true
			}
		}
	}
	if errs {
		return results, errors.New("dbsync: errors occurred during sync")
	}
	return results, nil
}

type dbInfo struct {
	db      piclib.Backend
	objects map[string]bool
}

func AllWay(path string, config int, dbs ...piclib.Backend) (results []string, err error) {
	infos := map[string]*dbInfo{}
	for _, db := range dbs {
		names, err := db.ListN(path, infinity)
		if err != nil {
			return nil, err
		}
		info := &dbInfo{db, make(map[string]bool)}
		for _, name := range names {
			info.objects[name] = true
		}
		infos[db.Name()] = info
	}

	errs := false
	for n1, info1 := range infos {
		for n2, info2 := range infos {
			for name, _ := range info1.objects {
				if !info2.objects[name] {
					results = append(results, fmt.Sprintf("sync from %v to %v: %v", n1, n2, name))
					if config&Cdry != 0 {
						continue
					}
					data, err := info1.db.Get(name)
					if err != nil {
						results = append(results, err.Error())
						errs = true
						continue
					}
					if err := info2.db.Put(name, data); err != nil {
						results = append(results, err.Error())
						errs = true
					}
				}
			}
		}
	}
	if errs {
		return results, errors.New("dbsync: errors occurred during sync")
	}
	return results, nil
}
