package piclib

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Name      string
	PassHash  string
	CanWrite  bool
	NotesRE   string
	HideEmpty bool
}

func HashPassword(pass string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, pass string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) == nil
}

func ValidNotesRE(re string) error {
	if re == "" {
		return nil
	}
	_, err := regexp.Compile(re)
	return err
}

func (u *User) Visible(notes string) bool {
	var re *regexp.Regexp
	if u.NotesRE != "" {
		var err error
		re, err = regexp.Compile(u.NotesRE)
		if err != nil {
			return false
		}
	}
	return u.visible(notes, re)
}

func (u *User) visible(notes string, re *regexp.Regexp) bool {
	notes = strings.TrimSpace(notes)
	if u.HideEmpty && notes == "" {
		return false
	}
	if re == nil {
		return true
	}
	return re.MatchString(notes)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanUser(s interface {
	Scan(dest ...any) error
}) (*User, error) {
	u := &User{}
	var write, hide int
	err := s.Scan(&u.Name, &u.PassHash, &write, &u.NotesRE, &hide)
	if err != nil {
		return nil, err
	}
	u.CanWrite = write != 0
	u.HideEmpty = hide != 0
	return u, nil
}

func (l *Lib) AddUser(u *User) error {
	if u.Name == "" {
		return fmt.Errorf("user name required")
	}
	if err := ValidNotesRE(u.NotesRE); err != nil {
		return fmt.Errorf("notes regex: %w", err)
	}
	_, err := l.db.Exec(
		"INSERT INTO users (name, pass_hash, can_write, notes_re, hide_empty) VALUES (?,?,?,?,?)",
		u.Name, u.PassHash, boolInt(u.CanWrite), u.NotesRE, boolInt(u.HideEmpty),
	)
	return err
}

func (l *Lib) PutUser(u *User) error {
	if u.Name == "" {
		return fmt.Errorf("user name required")
	}
	if err := ValidNotesRE(u.NotesRE); err != nil {
		return fmt.Errorf("notes regex: %w", err)
	}
	_, err := l.db.Exec(
		`INSERT INTO users (name, pass_hash, can_write, notes_re, hide_empty) VALUES (?,?,?,?,?)
		 ON CONFLICT(name) DO UPDATE SET
		 pass_hash=excluded.pass_hash,
		 can_write=excluded.can_write,
		 notes_re=excluded.notes_re,
		 hide_empty=excluded.hide_empty`,
		u.Name, u.PassHash, boolInt(u.CanWrite), u.NotesRE, boolInt(u.HideEmpty),
	)
	return err
}

func (l *Lib) GetUser(name string) (*User, error) {
	row := l.db.QueryRow(
		"SELECT name, pass_hash, can_write, notes_re, hide_empty FROM users WHERE name=?",
		name,
	)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (l *Lib) ListUsers() ([]*User, error) {
	rows, err := l.db.Query("SELECT name, pass_hash, can_write, notes_re, hide_empty FROM users ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (l *Lib) DeleteUser(name string) error {
	_, err := l.db.Exec("DELETE FROM users WHERE name=?", name)
	return err
}

func (l *Lib) UserCount() (int, error) {
	n := 0
	err := l.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

func (l *Lib) AuthUser(name, pass string) (*User, error) {
	u, err := l.GetUser(name)
	if err != nil {
		return nil, err
	}
	if u == nil || !CheckPassword(u.PassHash, pass) {
		return nil, nil
	}
	return u, nil
}
