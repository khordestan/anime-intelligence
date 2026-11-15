package store

// Remove restores the in-memory entry if persistence fails.
func (s *Store) Remove(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.library[id]
	if !ok {
		return false, nil
	}
	delete(s.library, id)
	if err := s.persistLocked(); err != nil {
		s.library[id] = e
		return false, err
	}
	return true, nil
}
