package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	CallTypeAudio = "audio"
	CallTypeVideo = "video"

	CallStatusRinging  = "ringing"
	CallStatusAccepted = "accepted"
	CallStatusRejected = "rejected"
	CallStatusEnded    = "ended"
	CallStatusMissed   = "missed"
)

type CallSession struct {
	ID             string     `json:"id"`
	CallerID       int        `json:"caller_id"`
	CallerUsername string     `json:"caller_username,omitempty"`
	CalleeID       int        `json:"callee_id"`
	CalleeUsername string     `json:"callee_username,omitempty"`
	CallType       string     `json:"call_type"`
	Status         string     `json:"status"`
	OfferSDP       string     `json:"offer_sdp,omitempty"`
	AnswerSDP      string     `json:"answer_sdp,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
}

type CallIceCandidate struct {
	ID             int64     `json:"id"`
	CallID         string    `json:"call_id"`
	SenderID       int       `json:"sender_id"`
	Candidate      string    `json:"candidate"`
	SDPMid         string    `json:"sdp_mid,omitempty"`
	SDPMLineIndex  *int      `json:"sdp_m_line_index,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func CreateCallSession(db *sql.DB, callerID, calleeID int, callType, offerSDP string) (*CallSession, error) {
	callType = strings.ToLower(strings.TrimSpace(callType))
	if callType != CallTypeAudio && callType != CallTypeVideo {
		callType = CallTypeAudio
	}
	offerSDP = strings.TrimSpace(offerSDP)
	if offerSDP == "" {
		return nil, errors.New("offer SDP is required")
	}

	if err := expireStaleCalls(db, callerID, calleeID); err != nil {
		return nil, err
	}

	callID := uuid.NewString()
	_, err := db.Exec(`
		INSERT INTO call_sessions (id, caller_id, callee_id, call_type, status, offer_sdp, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, callID, callerID, calleeID, callType, CallStatusRinging, offerSDP)
	if err != nil {
		return nil, err
	}

	return GetCallSessionByID(db, callID)
}

func expireStaleCalls(db *sql.DB, userA, userB int) error {
	_, err := db.Exec(`
		UPDATE call_sessions
		SET status = ?, ended_at = NOW(), updated_at = NOW()
		WHERE status IN (?, ?)
		  AND ((caller_id = ? AND callee_id = ?) OR (caller_id = ? AND callee_id = ?))
	`, CallStatusEnded, CallStatusRinging, CallStatusAccepted, userA, userB, userB, userA)
	return err
}

func GetCallSessionByID(db *sql.DB, callID string) (*CallSession, error) {
	row := db.QueryRow(`
		SELECT c.id, c.caller_id, caller.username, c.callee_id, callee.username,
		       c.call_type, c.status, COALESCE(c.offer_sdp, ''), COALESCE(c.answer_sdp, ''),
		       c.created_at, c.updated_at, c.ended_at
		FROM call_sessions c
		INNER JOIN users caller ON caller.id = c.caller_id
		INNER JOIN users callee ON callee.id = c.callee_id
		WHERE c.id = ?
	`, callID)

	call := &CallSession{}
	if err := row.Scan(
		&call.ID,
		&call.CallerID,
		&call.CallerUsername,
		&call.CalleeID,
		&call.CalleeUsername,
		&call.CallType,
		&call.Status,
		&call.OfferSDP,
		&call.AnswerSDP,
		&call.CreatedAt,
		&call.UpdatedAt,
		&call.EndedAt,
	); err != nil {
		return nil, err
	}
	return call, nil
}

func ListIncomingCalls(db *sql.DB, calleeID int) ([]CallSession, error) {
	if err := markMissedCalls(db, calleeID); err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT c.id, c.caller_id, caller.username, c.callee_id, callee.username,
		       c.call_type, c.status, COALESCE(c.offer_sdp, ''), COALESCE(c.answer_sdp, ''),
		       c.created_at, c.updated_at, c.ended_at
		FROM call_sessions c
		INNER JOIN users caller ON caller.id = c.caller_id
		INNER JOIN users callee ON callee.id = c.callee_id
		WHERE c.callee_id = ? AND c.status = ?
		ORDER BY c.created_at DESC
	`, calleeID, CallStatusRinging)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	calls := make([]CallSession, 0)
	for rows.Next() {
		call, err := scanCallSession(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, *call)
	}
	return calls, rows.Err()
}

func markMissedCalls(db *sql.DB, calleeID int) error {
	_, err := db.Exec(`
		UPDATE call_sessions
		SET status = ?, ended_at = NOW(), updated_at = NOW()
		WHERE callee_id = ?
		  AND status = ?
		  AND created_at < (NOW() - INTERVAL 45 SECOND)
	`, CallStatusMissed, calleeID, CallStatusRinging)
	return err
}

func AcceptCallSession(db *sql.DB, callID string, calleeID int, answerSDP string) (*CallSession, error) {
	answerSDP = strings.TrimSpace(answerSDP)
	if answerSDP == "" {
		return nil, errors.New("answer SDP is required")
	}

	result, err := db.Exec(`
		UPDATE call_sessions
		SET status = ?, answer_sdp = ?, updated_at = NOW()
		WHERE id = ? AND callee_id = ? AND status = ?
	`, CallStatusAccepted, answerSDP, callID, calleeID, CallStatusRinging)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, fmt.Errorf("call is not available to accept")
	}
	return GetCallSessionByID(db, callID)
}

func RejectCallSession(db *sql.DB, callID string, userID int) (*CallSession, error) {
	result, err := db.Exec(`
		UPDATE call_sessions
		SET status = ?, ended_at = NOW(), updated_at = NOW()
		WHERE id = ?
		  AND callee_id = ?
		  AND status = ?
	`, CallStatusRejected, callID, userID, CallStatusRinging)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, fmt.Errorf("call is not available to reject")
	}
	return GetCallSessionByID(db, callID)
}

func EndCallSession(db *sql.DB, callID string, userID int) (*CallSession, error) {
	result, err := db.Exec(`
		UPDATE call_sessions
		SET status = ?, ended_at = NOW(), updated_at = NOW()
		WHERE id = ?
		  AND (caller_id = ? OR callee_id = ?)
		  AND status IN (?, ?)
	`, CallStatusEnded, callID, userID, userID, CallStatusRinging, CallStatusAccepted)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return GetCallSessionByID(db, callID)
	}
	return GetCallSessionByID(db, callID)
}

func UserCanAccessCall(call *CallSession, userID int) bool {
	if call == nil {
		return false
	}
	return call.CallerID == userID || call.CalleeID == userID
}

func AddCallIceCandidate(db *sql.DB, callID string, senderID int, candidate, sdpMid string, sdpMLineIndex *int) (*CallIceCandidate, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return nil, errors.New("candidate is required")
	}

	result, err := db.Exec(`
		INSERT INTO call_ice_candidates (call_id, sender_id, candidate, sdp_mid, sdp_m_line_index, created_at)
		VALUES (?, ?, ?, ?, ?, NOW())
	`, callID, senderID, candidate, nullIfEmptyString(sdpMid), sdpMLineIndex)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetCallIceCandidateByID(db, id)
}

func nullIfEmptyString(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func GetCallIceCandidateByID(db *sql.DB, id int64) (*CallIceCandidate, error) {
	row := db.QueryRow(`
		SELECT id, call_id, sender_id, candidate, COALESCE(sdp_mid, ''), sdp_m_line_index, created_at
		FROM call_ice_candidates
		WHERE id = ?
	`, id)

	item := &CallIceCandidate{}
	var sdpMLineIndex sql.NullInt64
	if err := row.Scan(
		&item.ID,
		&item.CallID,
		&item.SenderID,
		&item.Candidate,
		&item.SDPMid,
		&sdpMLineIndex,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	if sdpMLineIndex.Valid {
		value := int(sdpMLineIndex.Int64)
		item.SDPMLineIndex = &value
	}
	return item, nil
}

func ListCallIceCandidates(db *sql.DB, callID string, sinceID int64) ([]CallIceCandidate, error) {
	if sinceID < 0 {
		sinceID = 0
	}

	rows, err := db.Query(`
		SELECT id, call_id, sender_id, candidate, COALESCE(sdp_mid, ''), sdp_m_line_index, created_at
		FROM call_ice_candidates
		WHERE call_id = ? AND id > ?
		ORDER BY id ASC
		LIMIT 200
	`, callID, sinceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CallIceCandidate, 0)
	for rows.Next() {
		item, err := scanCallIceCandidate(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanCallSession(scanner interface {
	Scan(dest ...any) error
}) (*CallSession, error) {
	call := &CallSession{}
	if err := scanner.Scan(
		&call.ID,
		&call.CallerID,
		&call.CallerUsername,
		&call.CalleeID,
		&call.CalleeUsername,
		&call.CallType,
		&call.Status,
		&call.OfferSDP,
		&call.AnswerSDP,
		&call.CreatedAt,
		&call.UpdatedAt,
		&call.EndedAt,
	); err != nil {
		return nil, err
	}
	return call, nil
}

func scanCallIceCandidate(scanner interface {
	Scan(dest ...any) error
}) (*CallIceCandidate, error) {
	item := &CallIceCandidate{}
	var sdpMLineIndex sql.NullInt64
	if err := scanner.Scan(
		&item.ID,
		&item.CallID,
		&item.SenderID,
		&item.Candidate,
		&item.SDPMid,
		&sdpMLineIndex,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	if sdpMLineIndex.Valid {
		value := int(sdpMLineIndex.Int64)
		item.SDPMLineIndex = &value
	}
	return item, nil
}
