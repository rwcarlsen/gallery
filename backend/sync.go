
package backend

import (
	"errors"
	"fmt"
	"bytes"
)

const (
	// print output without actually doing anything
	Sdry = 1 << iota
	// delete files at dst that don't exist at src (OneWay only)
	Sdel
)

const infinity = 500000

func SyncOneWay(path string, config int, from, to Interface) (results []string, err error) {
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
			if config&Sdry != 0 {
				continue
			}
			data, err := from.Get(objName)
			if err != nil {
				results = append(results, err.Error())
				errs = true
				continue
			}
			if err := to.Put(objName, bytes.NewReader(data)); err != nil {
				results = append(results, err.Error())
				errs = true
			}
		}
	}

	if config&Sdel != 0 {
		for objName, _ := range toObj {
			results = append(results, fmt.Sprintf("del at dst %v: %v", to.Name(), objName))
			if config&Sdry != 0 {
				continue
			}
			if !fromObj[objName] {
				if err := to.Del(objName); err != nil {
					results = append(results, err.Error())
					errs = true
				}
			}
		}
	}

	if errs {
		return results, errors.New("dbsync: errors occurred during sync")
	}
	return results, nil
}

type dbInfo struct {
	db      Interface
	objects map[string]bool
}

func SyncAllWay(path string, config int, dbs ...Interface) (results []string, err error) {
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
					if config&Sdry != 0 {
						continue
					}
					data, err := info1.db.Get(name)
					if err != nil {
						results = append(results, err.Error())
						errs = true
						continue
					}
					if err := info2.db.Put(name, bytes.NewReader(data)); err != nil {
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
