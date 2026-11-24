package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const meetingNotesLimit = 20000

var (
	ErrMeetingNotFound    = errors.New("meeting not found")
	ErrMeetingForbidden   = errors.New("you are not part of this meeting")
	ErrMeetingHostOnly    = errors.New("only the host can add people")
	ErrMeetingNoPeople    = errors.New("add at least one person")
	ErrMeetingFull        = errors.New("a meeting can include up to 8 people")
	ErrMeetingUnknownUser = errors.New("that person was not found")
	ErrMeetingTimeOrder   = errors.New("the end time must be after the start time")
)

type MeetingValidationError struct{ Msg string }

func (e *MeetingValidationError) Error() string { return e.Msg }

type MeetingPerson struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Meeting struct {
	ID           int             `json:"id"`
	HostID       int             `json:"host_id"`
	HostUsername string          `json:"host_username"`
	Title        string          `json:"title"`
	StartsAt     time.Time       `json:"starts_at"`
	EndsAt       *time.Time      `json:"ends_at,omitempty"`
	Notes        string          `json:"notes"`
	Participants []MeetingPerson `json:"participants"`
	CreatedAt    time.Time       `json:"created_at"`
}

func cleanMeetingTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", &MeetingValidationError{Msg: "Give the meeting a title."}
	}
	if utf8.RuneCountInString(title) > 200 {
		return "", &MeetingValidationError{Msg: "The title must be 200 characters or less."}
	}
	return title, nil
}

func cleanMeetingNotes(notes string) (string, error) {
	notes = strings.TrimSpace(notes)
	if utf8.RuneCountInString(notes) > meetingNotesLimit {
		return "", &MeetingValidationError{Msg: "Notes must be 20,000 characters or less."}
	}
	return notes, nil
}

func CreateMeeting(db *sql.DB, hostID int, title string, startsAt time.Time, endsAt *time.Time, notes string, userIDs []int) (*Meeting, error) {
	title, err := cleanMeetingTitle(title)
	if err != nil {
		return nil, err
	}
	notes, err = cleanMeetingNotes(notes)
	if err != nil {
		return nil, err
	}
	if startsAt.IsZero() {
		return nil, &MeetingValidationError{Msg: "Choose a start time."}
	}
	if endsAt != nil && !endsAt.After(startsAt) {
		return nil, ErrMeetingTimeOrder
	}
	people := uniquePositiveIDs(userIDs, hostID)
	if len(people) == 0 {
		return nil, ErrMeetingNoPeople
	}
	if len(people)+1 > GroupCallMaxParticipants {
		return nil, ErrMeetingFull
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if n, err := countExistingUsers(tx, people); err != nil {
		return nil, err
	} else if n != len(people) {
		return nil, ErrMeetingUnknownUser
	}

	res, err := tx.Exec(`
		INSERT INTO meetings (host_id, title, starts_at, ends_at, notes)
		VALUES (?, ?, ?, ?, ?)`, hostID, title, startsAt.UTC(), nullableTime(endsAt), notes)
	if err != nil {
		return nil, err
	}
	id64, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	for _, uid := range people {
		if _, err := tx.Exec(`INSERT INTO meeting_participants (meeting_id, user_id) VALUES (?, ?)`, id64, uid); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetMeeting(db, int(id64))
}

func nullableTime(value *time.Time) interface{} {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func ListMeetings(db *sql.DB, userID int) ([]Meeting, error) {
	rows, err := db.Query(`
		SELECT m.id
		FROM meetings m
		LEFT JOIN meeting_participants p ON p.meeting_id = m.id
		WHERE m.host_id = ? OR p.user_id = ?
		GROUP BY m.id, m.starts_at
		ORDER BY m.starts_at DESC
		LIMIT 100`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	meetings := make([]Meeting, 0, len(ids))
	for _, id := range ids {
		meeting, err := GetMeeting(db, id)
		if err != nil {
			return nil, err
		}
		meetings = append(meetings, *meeting)
	}
	return meetings, nil
}

func GetMeeting(db *sql.DB, id int) (*Meeting, error) {
	meeting := &Meeting{Participants: []MeetingPerson{}}
	var notes sql.NullString
	var ends sql.NullTime
	err := db.QueryRow(`
		SELECT m.id, m.host_id, u.username, m.title, m.starts_at, m.ends_at, m.notes, m.created_at
		FROM meetings m INNER JOIN users u ON u.id = m.host_id
		WHERE m.id = ?`, id).
		Scan(&meeting.ID, &meeting.HostID, &meeting.HostUsername, &meeting.Title, &meeting.StartsAt, &ends, &notes, &meeting.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMeetingNotFound
	}
	if err != nil {
		return nil, err
	}
	if ends.Valid {
		t := ends.Time
		meeting.EndsAt = &t
	}
	if notes.Valid {
		meeting.Notes = notes.String
	}
	rows, err := db.Query(`
		SELECT p.user_id, u.username, u.email
		FROM meeting_participants p INNER JOIN users u ON u.id = p.user_id
		WHERE p.meeting_id = ?
		ORDER BY u.username`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var person MeetingPerson
		if err := rows.Scan(&person.UserID, &person.Username, &person.Email); err != nil {
			return nil, err
		}
		meeting.Participants = append(meeting.Participants, person)
	}
	return meeting, rows.Err()
}

func UserCanAccessMeeting(db *sql.DB, meetingID, userID int) (bool, error) {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM meetings m
		LEFT JOIN meeting_participants p ON p.meeting_id = m.id AND p.user_id = ?
		WHERE m.id = ? AND (m.host_id = ? OR p.user_id = ?)`, userID, meetingID, userID, userID).Scan(&n)
	return n > 0, err
}

func AddMeetingParticipant(db *sql.DB, meetingID, hostID, userID int) (*Meeting, error) {
	meeting, err := GetMeeting(db, meetingID)
	if err != nil {
		return nil, err
	}
	if meeting.HostID != hostID {
		return nil, ErrMeetingHostOnly
	}
	if userID == hostID {
		return meeting, nil
	}
	for _, person := range meeting.Participants {
		if person.UserID == userID {
			return meeting, nil
		}
	}
	if len(meeting.Participants)+1 >= GroupCallMaxParticipants {
		return nil, ErrMeetingFull
	}
	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, userID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrMeetingUnknownUser
	}
	if _, err := db.Exec(`INSERT INTO meeting_participants (meeting_id, user_id) VALUES (?, ?)`, meetingID, userID); err != nil {
		return nil, err
	}
	return GetMeeting(db, meetingID)
}

func UpdateMeetingNotes(db *sql.DB, meetingID, userID int, notes string) (*Meeting, error) {
	notes, err := cleanMeetingNotes(notes)
	if err != nil {
		return nil, err
	}
	allowed, err := UserCanAccessMeeting(db, meetingID, userID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrMeetingForbidden
	}
	res, err := db.Exec(`UPDATE meetings SET notes = ? WHERE id = ?`, notes, meetingID)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		ok, err := UserCanAccessMeeting(db, meetingID, userID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrMeetingNotFound
		}
	}
	return GetMeeting(db, meetingID)
}
