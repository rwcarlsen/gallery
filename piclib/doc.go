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
//   * users
//   	- name TEXT (primary key)
//   	- pass_hash TEXT (bcrypt)
//   	- can_write INTEGER (0=read, 1=read+write notes)
//   	- notes_re TEXT (Go regex on latest Notes; empty = no regex filter)
//   	- hide_empty INTEGER (1=hide pics with blank notes)
package piclib

// TODO: mount groups of pics named nicely (maybe in nice dir structure) with
// softlinks
