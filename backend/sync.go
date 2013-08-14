package backend

import (
	"errors"
	"fmt"
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
	names, err := from.Enumerate(path, infinity)
	if err != nil {
		return nil, err
	}

	fromObj := make(map[string]bool)
	for _, name := range names {
		fromObj[name] = true
	}

	names, err = to.Enumerate(path, infinity)
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
			results = append(results, fmt.Sprintf("sync: %v", objName))
			if config&SyncDry != 0 {
				continue
			}
			rc, err := from.Get(objName)
			if err != nil {
				results = append(results, err.Error())
				errs = true
				continue
			}

			if _, err := to.Put(objName, rc); err != nil {
				results = append(results, err.Error())
				errs = true
			}
			rc.Close()
		}
	}

	if config&SyncDel != 0 {
		for objName, _ := range toObj {
			if !fromObj[objName] {
				results = append(results, fmt.Sprintf("del at dst: %v", objName))
				if config&SyncDry != 0 {
					continue
				}
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
