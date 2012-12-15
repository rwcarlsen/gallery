
package backend

import (
	"errors"
	"fmt"
	"bytes"
)

const (
	// SyncStd is the default syncing configuration
	SyncStd = 0
	// SyncDry causes output to be printed without actually doing anything.
	SyncDry = 1 << iota
	// SyncDel causes files at dst that don't exist at src to be deleted
	// (OneWay only).
	SyncDel
)

const infinity = 500000

// SyncOneWay syncs path recursively between backends 'from' and 'to' according
// to specified config.  Returned results contains an entry for each sync
// operation (including errors if any).
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
			if config&SyncDry != 0 {
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

	if config&SyncDel != 0 {
		for objName, _ := range toObj {
			results = append(results, fmt.Sprintf("del at dst %v: %v", to.Name(), objName))
			if config&SyncDry != 0 {
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

// SyncAllWay syncs all dbs between eachother write-only (no deletions)
// according to specified config.  Returned results contains an entry for each
// sync operation (including errors if any).
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
					if config&SyncDry != 0 {
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

