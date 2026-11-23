package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	GroupCallStatusActive = "active"
	GroupCallStatusEnded  = "ended"

	GroupParticipantInvited  = "invited"
	GroupParticipantJoined   = "joined"
	GroupParticipantLeft     = "left"
	GroupParticipantDeclined = "declined"
	GroupParticipantMissed   = "missed"

	// GroupCallMaxParticipants caps the mesh: every person sends media to every other person.
	GroupCallMaxParticipants = 8
	// GroupCallRingSeconds is how long an invitation rings before it counts as missed.
	GroupCallRingSeconds = 45
	// GroupCallStaleSeconds drops joined participants whose browser stopped syncing.
	GroupCallStaleSeconds = 30
	// GroupCallMaxSignalBytes bounds one SDP/ICE payload.
	GroupCallMaxSignalBytes = 64 << 10
)

var (
	ErrGroupCallNotFound     = errors.New("group call not found")
	ErrGroupCallEnded        = errors.New("this group call has ended")
	ErrGroupCallFull         = errors.New("a group call can have up to 8 people")
	ErrGroupCallNotJoined    = errors.New("join the call first")
	ErrGroupCallNoInvitees   = errors.New("choose at least one person to call")
	ErrGroupCallBadSignal    = errors.New("invalid call signal")
	ErrGroupCallUnknownUsers = errors.New("one or more users were not found")
)

type GroupCallParticipant struct {
	UserID    int        `json:"user_id"`
	Username  string     `json:"username"`
	Status    string     `json:"status"`
	InvitedAt time.Time  `json:"invited_at"`
	JoinedAt  *time.Time `json:"joined_at,omitempty"`
	ImageURL  string     `json:"image_url,omitempty"`
}

type GroupCall struct {
	ID           string                 `json:"id"`
	HostID       int                    `json:"host_id"`
	HostUsername string                 `json:"host_username"`
	CallType     string                 `json:"call_type"`
	Status       string                 `json:"status"`
	CreatedAt    time.Time              `json:"created_at"`
	EndedAt      *time.Time             `json:"ended_at,omitempty"`
	Participants []GroupCallParticipant `json:"participants"`
}

type GroupCallSignal struct {
	ID         int64     `json:"id"`
	FromUserID int       `json:"from_user_id"`
	Kind       string    `json:"kind"`
	Payload    string    `json:"payload"`
	CreatedAt  time.Time `json:"created_at"`
}

// Participant returns the participant row for userID, or nil.
func (g *GroupCall) Participant(userID int) *GroupCallParticipant {
	for i := range g.Participants {
		if g.Participants[i].UserID == userID {
			return &g.Participants[i]
		}
	}
	return nil
}

func uniquePositiveIDs(ids []int, exclude int) []int {
	seen := map[int]bool{exclude: true}
	out := []int{}
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func countExistingUsers(tx *sql.Tx, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE id IN (`+placeholders+`)`, args...).Scan(&n)
	return n, err
}

// CreateGroupCall starts a call with the host joined and everyone else invited.
func CreateGroupCall(db *sql.DB, hostID int, inviteeIDs []int, callType string) (*GroupCall, error) {
	invitees := uniquePositiveIDs(inviteeIDs, hostID)
	if len(invitees) == 0 {
		return nil, ErrGroupCallNoInvitees
	}
	if len(invitees)+1 > GroupCallMaxParticipants {
		return nil, ErrGroupCallFull
	}
	callType = strings.ToLower(strings.TrimSpace(callType))
	if callType != CallTypeVideo {
		callType = CallTypeAudio
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if n, err := countExistingUsers(tx, invitees); err != nil {
		return nil, err
	} else if n != len(invitees) {
		return nil, ErrGroupCallUnknownUsers
	}

	id := uuid.NewString()
	if _, err := tx.Exec(`INSERT INTO group_calls (id, host_id, call_type, status, created_at) VALUES (?, ?, ?, ?, NOW())`,
		id, hostID, callType, GroupCallStatusActive); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`
		INSERT INTO group_call_participants (call_id, user_id, status, invited_by, invited_at, joined_at, last_seen_at)
		VALUES (?, ?, ?, ?, NOW(), NOW(), NOW())`, id, hostID, GroupParticipantJoined, hostID); err != nil {
		return nil, err
	}
	for _, uid := range invitees {
		if _, err := tx.Exec(`
			INSERT INTO group_call_participants (call_id, user_id, status, invited_by, invited_at)
			VALUES (?, ?, ?, ?, NOW())`, id, uid, GroupParticipantInvited, hostID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetGroupCall(db, id)
}

// GetGroupCall loads a call and its participants.
func GetGroupCall(db *sql.DB, callID string) (*GroupCall, error) {
	call := &GroupCall{Participants: []GroupCallParticipant{}}
	err := db.QueryRow(`
		SELECT g.id, g.host_id, u.username, g.call_type, g.status, g.created_at, g.ended_at
		FROM group_calls g INNER JOIN users u ON u.id = g.host_id
		WHERE g.id = ?`, callID).
		Scan(&call.ID, &call.HostID, &call.HostUsername, &call.CallType, &call.Status, &call.CreatedAt, &call.EndedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGroupCallNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT p.user_id, u.username, p.status, p.invited_at, p.joined_at, COALESCE(u.profile_photo_url, '')
		FROM group_call_participants p INNER JOIN users u ON u.id = p.user_id
		WHERE p.call_id = ?
		ORDER BY p.joined_at IS NULL, p.joined_at, u.username`, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p GroupCallParticipant
		if err := rows.Scan(&p.UserID, &p.Username, &p.Status, &p.InvitedAt, &p.JoinedAt, &p.ImageURL); err != nil {
			return nil, err
		}
		p.ImageURL = ProfilePhotoForClient(p.ImageURL)
		call.Participants = append(call.Participants, p)
	}
	return call, rows.Err()
}

// expireGroupCallState marks unanswered invitations as missed, drops joined participants
// that stopped syncing, and ends calls nobody is in any more.
func expireGroupCallState(db *sql.DB, callID string) error {
	if _, err := db.Exec(`
		UPDATE group_call_participants SET status = ?
		WHERE call_id = ? AND status = ? AND invited_at < NOW() - INTERVAL ? SECOND`,
		GroupParticipantMissed, callID, GroupParticipantInvited, GroupCallRingSeconds); err != nil {
		return err
	}
	if _, err := db.Exec(`
		UPDATE group_call_participants SET status = ?, left_at = NOW()
		WHERE call_id = ? AND status = ? AND last_seen_at < NOW() - INTERVAL ? SECOND`,
		GroupParticipantLeft, callID, GroupParticipantJoined, GroupCallStaleSeconds); err != nil {
		return err
	}
	return endGroupCallIfEmpty(db, callID)
}

func endGroupCallIfEmpty(db *sql.DB, callID string) error {
	var joined int
	if err := db.QueryRow(`SELECT COUNT(*) FROM group_call_participants WHERE call_id = ? AND status = ?`,
		callID, GroupParticipantJoined).Scan(&joined); err != nil {
		return err
	}
	if joined > 0 {
		return nil
	}
	if _, err := db.Exec(`UPDATE group_calls SET status = ?, ended_at = NOW() WHERE id = ? AND status = ?`,
		GroupCallStatusEnded, callID, GroupCallStatusActive); err != nil {
		return err
	}
	// Once ended, nobody should keep ringing.
	_, err := db.Exec(`UPDATE group_call_participants SET status = ? WHERE call_id = ? AND status = ?`,
		GroupParticipantMissed, callID, GroupParticipantInvited)
	return err
}

// ListIncomingGroupCalls returns active calls that are ringing for userID.
func ListIncomingGroupCalls(db *sql.DB, userID int) ([]GroupCall, error) {
	rows, err := db.Query(`
		SELECT p.call_id FROM group_call_participants p
		INNER JOIN group_calls g ON g.id = p.call_id
		WHERE p.user_id = ? AND p.status = ? AND g.status = ?
		  AND p.invited_at >= NOW() - INTERVAL ? SECOND
		ORDER BY p.invited_at DESC LIMIT 5`,
		userID, GroupParticipantInvited, GroupCallStatusActive, GroupCallRingSeconds)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()

	calls := []GroupCall{}
	for _, id := range ids {
		if err := expireGroupCallState(db, id); err != nil {
			return nil, err
		}
		call, err := GetGroupCall(db, id)
		if err != nil {
			continue
		}
		if p := call.Participant(userID); call.Status == GroupCallStatusActive && p != nil && p.Status == GroupParticipantInvited {
			calls = append(calls, *call)
		}
	}
	return calls, nil
}

// JoinGroupCall moves an invited (or previously left) participant into the call.
// It returns the signal cursor: signals with a higher id are meant for this session.
func JoinGroupCall(db *sql.DB, callID string, userID int) (*GroupCall, int64, error) {
	if err := expireGroupCallState(db, callID); err != nil {
		return nil, 0, err
	}
	call, err := GetGroupCall(db, callID)
	if err != nil {
		return nil, 0, err
	}
	if call.Status != GroupCallStatusActive {
		return nil, 0, ErrGroupCallEnded
	}
	if call.Participant(userID) == nil {
		return nil, 0, ErrGroupCallNotFound
	}

	var cursor int64
	if err := db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM group_call_signals WHERE call_id = ?`, callID).Scan(&cursor); err != nil {
		return nil, 0, err
	}
	// Old signals addressed to this user belong to a previous session.
	if _, err := db.Exec(`DELETE FROM group_call_signals WHERE call_id = ? AND (to_user_id = ? OR from_user_id = ?)`,
		callID, userID, userID); err != nil {
		return nil, 0, err
	}
	if _, err := db.Exec(`
		UPDATE group_call_participants
		SET status = ?, joined_at = NOW(), last_seen_at = NOW(), left_at = NULL
		WHERE call_id = ? AND user_id = ?`, GroupParticipantJoined, callID, userID); err != nil {
		return nil, 0, err
	}
	call, err = GetGroupCall(db, callID)
	return call, cursor, err
}

// DeclineGroupCall stops the invitation ringing for userID.
func DeclineGroupCall(db *sql.DB, callID string, userID int) error {
	_, err := db.Exec(`UPDATE group_call_participants SET status = ? WHERE call_id = ? AND user_id = ? AND status = ?`,
		GroupParticipantDeclined, callID, userID, GroupParticipantInvited)
	return err
}

// LeaveGroupCall removes userID from the call and ends it when nobody is left.
func LeaveGroupCall(db *sql.DB, callID string, userID int) error {
	if _, err := db.Exec(`
		UPDATE group_call_participants SET status = ?, left_at = NOW()
		WHERE call_id = ? AND user_id = ? AND status = ?`,
		GroupParticipantLeft, callID, userID, GroupParticipantJoined); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM group_call_signals WHERE call_id = ? AND (to_user_id = ? OR from_user_id = ?)`,
		callID, userID, userID); err != nil {
		return err
	}
	return endGroupCallIfEmpty(db, callID)
}

// InviteToGroupCall rings more people. Only joined participants can invite.
func InviteToGroupCall(db *sql.DB, callID string, inviterID int, userIDs []int) (*GroupCall, error) {
	call, err := GetGroupCall(db, callID)
	if err != nil {
		return nil, err
	}
	if call.Status != GroupCallStatusActive {
		return nil, ErrGroupCallEnded
	}
	if p := call.Participant(inviterID); p == nil || p.Status != GroupParticipantJoined {
		return nil, ErrGroupCallNotJoined
	}

	toAdd := []int{}
	for _, uid := range uniquePositiveIDs(userIDs, inviterID) {
		if p := call.Participant(uid); p != nil && (p.Status == GroupParticipantJoined || p.Status == GroupParticipantInvited) {
			continue
		}
		toAdd = append(toAdd, uid)
	}
	if len(toAdd) == 0 {
		return call, nil
	}
	active := 0
	for _, p := range call.Participants {
		if p.Status == GroupParticipantJoined || p.Status == GroupParticipantInvited {
			active++
		}
	}
	if active+len(toAdd) > GroupCallMaxParticipants {
		return nil, ErrGroupCallFull
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if n, err := countExistingUsers(tx, toAdd); err != nil {
		return nil, err
	} else if n != len(toAdd) {
		return nil, ErrGroupCallUnknownUsers
	}
	for _, uid := range toAdd {
		if _, err := tx.Exec(`
			INSERT INTO group_call_participants (call_id, user_id, status, invited_by, invited_at)
			VALUES (?, ?, ?, ?, NOW())
			ON DUPLICATE KEY UPDATE status = VALUES(status), invited_by = VALUES(invited_by), invited_at = NOW(), left_at = NULL`,
			callID, uid, GroupParticipantInvited, inviterID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetGroupCall(db, callID)
}

// SyncGroupCall records a heartbeat for userID and returns the call plus new signals for them.
func SyncGroupCall(db *sql.DB, callID string, userID int, sinceID int64) (*GroupCall, []GroupCallSignal, error) {
	if _, err := db.Exec(`UPDATE group_call_participants SET last_seen_at = NOW() WHERE call_id = ? AND user_id = ? AND status = ?`,
		callID, userID, GroupParticipantJoined); err != nil {
		return nil, nil, err
	}
	if err := expireGroupCallState(db, callID); err != nil {
		return nil, nil, err
	}
	call, err := GetGroupCall(db, callID)
	if err != nil {
		return nil, nil, err
	}
	if call.Participant(userID) == nil {
		return nil, nil, ErrGroupCallNotFound
	}

	rows, err := db.Query(`
		SELECT id, from_user_id, kind, payload, created_at FROM group_call_signals
		WHERE call_id = ? AND to_user_id = ? AND id > ?
		ORDER BY id ASC LIMIT 300`, callID, userID, sinceID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	signals := []GroupCallSignal{}
	for rows.Next() {
		var s GroupCallSignal
		if err := rows.Scan(&s.ID, &s.FromUserID, &s.Kind, &s.Payload, &s.CreatedAt); err != nil {
			return nil, nil, err
		}
		signals = append(signals, s)
	}
	return call, signals, rows.Err()
}

// AddGroupCallSignal stores an offer, answer or ICE candidate from one joined participant to another.
func AddGroupCallSignal(db *sql.DB, callID string, fromID, toID int, kind, payload string) error {
	switch kind {
	case "offer", "answer", "candidate":
	default:
		return ErrGroupCallBadSignal
	}
	if fromID == toID || payload == "" || len(payload) > GroupCallMaxSignalBytes {
		return ErrGroupCallBadSignal
	}
	var joined int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM group_call_participants p INNER JOIN group_calls g ON g.id = p.call_id
		WHERE p.call_id = ? AND p.user_id IN (?, ?) AND p.status = ? AND g.status = ?`,
		callID, fromID, toID, GroupParticipantJoined, GroupCallStatusActive).Scan(&joined); err != nil {
		return err
	}
	if joined != 2 {
		return ErrGroupCallNotJoined
	}
	_, err := db.Exec(`INSERT INTO group_call_signals (call_id, from_user_id, to_user_id, kind, payload, created_at) VALUES (?, ?, ?, ?, ?, NOW())`,
		callID, fromID, toID, kind, payload)
	return err
}
