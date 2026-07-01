package session

import "time"

type State string

const (
	StateChoosingStyles State = "choosing_styles"
	StateCompleted      State = "completed"
)

type Session struct {
	GroupID            string
	UserID             string
	State              State
	GeneratedTemplates map[string]string
	GeneratedExpiresAt time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func New(groupID, userID string, now time.Time) *Session {
	return &Session{
		GroupID:   groupID,
		UserID:    userID,
		State:     StateChoosingStyles,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (s *Session) Key() string {
	return Key(s.GroupID, s.UserID)
}

func Key(groupID, userID string) string {
	return groupID + ":" + userID
}
