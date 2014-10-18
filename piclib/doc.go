// Package piclib provides tools backend-agnostic management of large photo collections.
//
// A SQLite database file with storing meta-data for the pictures/files is
// created with the following schema:
//
//   * files
//   	- id INTEGER
//   	- sum BLOB (sha256 bytes)
//   	- name TEXT (original filename)
//   	- added INTEGER (unix secs since epoch)
//   	- taken INTEGER (unix secs since epoch)
//   	- orient INTEGER (EXIF)
//   	- thumb BLOB (JPEG bytes)
//   * meta
//   	- id INTEGER (key into files table id)
//   	- time INTEGER (unix secs since epoch)
//   	- field TEXT
//   	- value TEXT
package piclib

// TODO: mount groups of pics named nicely (maybe in nice dir structure) with
// softlinks
