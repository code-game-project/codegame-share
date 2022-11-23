package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Bananenpro/log"
	"github.com/jmoiron/sqlx"
	gonanoid "github.com/matoous/go-nanoid/v2"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

const EntryLifespan time.Duration = 24 * time.Hour

var ErrNotFound = errors.New("not-found")

type EntryType int

const (
	TypeGame EntryType = iota
	TypeSpectate
	TypeSession
)

func connectDB(connectionString string) (*sqlx.DB, error) {
	return sqlx.Connect("sqlite3", connectionString)
}

type entry struct {
	Id           string    `db:"id"`
	Created      int64     `db:"created"`
	Type         EntryType `db:"type"`
	PasswordHash []byte    `db:"password_hash"`
	Data         []byte    `db:"data"`
}

func (s *Server) storeEntry(entryType EntryType, password string, data any) (string, error) {
	var passwordHash []byte
	if password != "" {
		var err error
		passwordHash, err = bcrypt.GenerateFromPassword([]byte(password), 10)
		if err != nil {
			log.Errorf("Failed to generate password hash: %s", err)
			return "", errors.New("failed to generate password hash")
		}
	}
	_, err := s.DB.Exec("DELETE FROM entries WHERE created < ?", time.Now().Add(-EntryLifespan).Unix())
	if err != nil {
		log.Errorf("Failed to delete expired entries: %s", err)
	}
	id := gonanoid.MustGenerate("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", 8)
	created := time.Now().Unix()

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Errorf("Failed to encode entry data: %s", err)
		return "", err
	}

	_, err = s.DB.Exec("INSERT INTO entries (id, created, type, password_hash, data) VALUES (?, ?, ?, ?, ?)", id, created, entryType, passwordHash, jsonData)
	if err != nil {
		log.Errorf("Failed to store entry: %s", err)
	}
	return id, err
}

func (s *Server) getEntry(id string) (entry, error) {
	_, err := s.DB.Exec("DELETE FROM entries WHERE created < ?", time.Now().Add(-EntryLifespan).Unix())
	if err != nil {
		log.Errorf("Failed to delete expired entries: %s", err)
	}

	var e entry
	err = s.DB.Get(&e, "SELECT * FROM entries WHERE id = ?", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entry{}, ErrNotFound
		} else {
			log.Errorf("Failed to find entry by id: %s", err)
			return entry{}, err
		}
	}
	return e, nil
}

func (s *Server) deleteEntry(id string) error {
	_, err := s.DB.Exec("DELETE FROM entries WHERE id = ?", id)
	if err != nil {
		log.Errorf("Failed to delete entry: %s", err)
	}
	return err
}
