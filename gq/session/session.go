package session

import (
	"os"
	"path/filepath"
	"time"

	"github.com/jkf16m/gq/config"
)

// New creates a new session and saves it.
func New(cfg *config.Config, id string) (*Session, error) {
	sess := newSession(id)
	
	if err := os.MkdirAll(cfg.SessionDir, 0700); err != nil {
		return nil, err
	}
	
	path := filepath.Join(cfg.SessionDir, id+".json")
	if err := saveSessionFile(sess, path); err != nil {
		return nil, err
	}
	
	return sess, nil
}

// Load loads a session by ID.
func Load(cfg *config.Config, id string) (*Session, error) {
	path := filepath.Join(cfg.SessionDir, id+".json")
	return loadSessionFile(path)
}

// List returns all session IDs.
func List(cfg *config.Config) ([]string, error) {
	paths, err := listSessionFiles(cfg.SessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	
	var ids []string
	for _, p := range paths {
		id := filepath.Base(p)
		id = id[:len(id)-5] // remove .json
		ids = append(ids, id)
	}
	return ids, nil
}

// AddMessage adds a message to a session and saves it.
func AddMessage(cfg *config.Config, sess *Session, msg Message) error {
	sess.Messages = append(sess.Messages, msg)
	sess.Updated = time.Now()
	
	path := filepath.Join(cfg.SessionDir, sess.ID+".json")
	return saveSessionFile(sess, path)
}

// Last returns the most recently updated session, or nil if none.
func Last(cfg *config.Config) (*Session, error) {
	ids, err := List(cfg)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	
	var newest *Session
	for _, id := range ids {
		sess, err := Load(cfg, id)
		if err != nil {
			continue
		}
		if newest == nil || sess.Updated.After(newest.Updated) {
			newest = sess
		}
	}
	
	return newest, nil
}