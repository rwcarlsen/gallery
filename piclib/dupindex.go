package piclib

import (
	"crypto"
	_ "crypto/sha256"
	"encoding/json"
	"errors"
)

type dupIndex struct {
	// map[hash]metaName
	Hashes map[string]string
}

func (di *dupIndex) NotifyAdd(p *Photo, data []byte) (err error, dupName string) {
	if dup, name := di.IsDup(data); dup {
		return errors.New("piclib: pic has duplicate"), name
	}

	sum := di.hash(data)
	di.Hashes[sum] = p.Meta
	return nil, ""
}

func (di *dupIndex) hash(data []byte) string {
	h := crypto.SHA256.New()
	return string(h.Sum([]byte{}))
}

func (di *dupIndex) IsDup(data []byte) (bool, string) {
	sum := di.hash(data)
	meta, ok := di.Hashes[sum]
	return ok, meta
}

func (di *dupIndex) Dump() []byte {
	data, err := json.Marshal(di)
	if err != nil {
		panic(err.Error())
	}
	return data
}

func (di *dupIndex) Load(data []byte) {
	err := json.Unmarshal(data, di)
	if err != nil {
		panic(err.Error())
	}
}
