package session

func (s *Session) UserID() string {
	return s.GetString(userKey)
}

func (s *Session) SetUserID(id string) {
	s.Set(userKey, id)
}

func (s *Session) ClearUser() {
	s.Delete(userKey)
}
