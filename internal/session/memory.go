package session

import "sync"

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]*Session)}
}

func (s *MemoryStore) Get(groupID, userID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[Key(groupID, userID)]
	return session, ok
}

func (s *MemoryStore) Save(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.Key()] = session
}

func (s *MemoryStore) Delete(groupID, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, Key(groupID, userID))
}
