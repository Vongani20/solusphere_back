package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	TaskStatusTodo       = "todo"
	TaskStatusInProgress = "in_progress"
	TaskStatusDone       = "done"

	TaskPriorityLow    = "low"
	TaskPriorityMedium = "medium"
	TaskPriorityHigh   = "high"
)

var (
	ErrTaskNotFound  = errors.New("task not found")
	ErrTaskForbidden = errors.New("you can't change this task")
	ErrTaskInvalid   = errors.New("invalid task")
)

// TaskValidationError carries a user-facing message.
type TaskValidationError struct{ Msg string }

func (e *TaskValidationError) Error() string { return e.Msg }
func (e *TaskValidationError) Unwrap() error { return ErrTaskInvalid }

type Task struct {
	ID               int        `json:"id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Status           string     `json:"status"`
	Priority         string     `json:"priority"`
	DueDate          string     `json:"due_date"`
	Overdue          bool       `json:"overdue"`
	CreatedBy        int        `json:"created_by"`
	CreatedByName    string     `json:"created_by_name"`
	AssignedTo       int        `json:"assigned_to"`
	AssignedToName   string     `json:"assigned_to_name"`
	AssignedToImage  string     `json:"assigned_to_image_url,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	CanEdit          bool       `json:"can_edit"`
	CanChangeStatus  bool       `json:"can_change_status"`
}

// TaskInput is used for create and update. Nil pointers mean "leave unchanged".
type TaskInput struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	DueDate     *string `json:"due_date"`
	AssignedTo  *int    `json:"assigned_to"`
}

type TaskSummary struct {
	Open     int `json:"open"`
	Overdue  int `json:"overdue"`
	DueToday int `json:"due_today"`
}

func validStatus(s string) bool {
	return s == TaskStatusTodo || s == TaskStatusInProgress || s == TaskStatusDone
}

func validPriority(p string) bool {
	return p == TaskPriorityLow || p == TaskPriorityMedium || p == TaskPriorityHigh
}

// normalizeTaskInput trims fields and validates the ones that are set.
func normalizeTaskInput(in *TaskInput) error {
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" {
			return &TaskValidationError{"Give the task a title."}
		}
		if utf8.RuneCountInString(t) > 200 {
			return &TaskValidationError{"Keep the title under 200 characters."}
		}
		in.Title = &t
	}
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		if utf8.RuneCountInString(d) > 4000 {
			return &TaskValidationError{"Keep the description under 4000 characters."}
		}
		in.Description = &d
	}
	if in.Status != nil && !validStatus(*in.Status) {
		return &TaskValidationError{"Status must be To do, In progress or Done."}
	}
	if in.Priority != nil && !validPriority(*in.Priority) {
		return &TaskValidationError{"Priority must be Low, Medium or High."}
	}
	if in.DueDate != nil {
		d := strings.TrimSpace(*in.DueDate)
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return &TaskValidationError{"Use a valid due date."}
			}
		}
		in.DueDate = &d
	}
	return nil
}

const taskSelect = `
	SELECT t.id, t.title, COALESCE(t.description, ''), t.status, t.priority, t.due_date,
	       t.created_by, creator.username, t.assigned_to, assignee.username,
	       COALESCE(assignee.profile_photo_url, ''), t.created_at, t.updated_at, t.completed_at
	FROM tasks t
	INNER JOIN users creator ON creator.id = t.created_by
	INNER JOIN users assignee ON assignee.id = t.assigned_to`

func scanTask(row interface{ Scan(...any) error }, viewerID int, viewerIsAdmin bool) (*Task, error) {
	var t Task
	var due sql.NullTime
	if err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority, &due,
		&t.CreatedBy, &t.CreatedByName, &t.AssignedTo, &t.AssignedToName, &t.AssignedToImage,
		&t.CreatedAt, &t.UpdatedAt, &t.CompletedAt); err != nil {
		return nil, err
	}
	if due.Valid {
		t.DueDate = due.Time.Format("2006-01-02")
		today := time.Now().Format("2006-01-02")
		t.Overdue = t.Status != TaskStatusDone && t.DueDate < today
	}
	t.AssignedToImage = ProfilePhotoForClient(t.AssignedToImage)
	t.CanEdit = viewerIsAdmin || viewerID == t.CreatedBy
	t.CanChangeStatus = t.CanEdit || viewerID == t.AssignedTo
	return &t, nil
}

// ListTasks returns tasks for a scope: "mine" (assigned to me), "created" (I created), "all" (admins).
func ListTasks(db *sql.DB, viewerID int, viewerIsAdmin bool, scope string) ([]Task, error) {
	where, args := "t.assigned_to = ?", []interface{}{viewerID}
	switch scope {
	case "created":
		where = "t.created_by = ?"
	case "all":
		if !viewerIsAdmin {
			return nil, ErrTaskForbidden
		}
		where, args = "1 = 1", nil
	}
	rows, err := db.Query(taskSelect+` WHERE `+where+`
		ORDER BY FIELD(t.status, 'todo', 'in_progress', 'done'),
		         t.due_date IS NULL, t.due_date,
		         FIELD(t.priority, 'high', 'medium', 'low'), t.id DESC
		LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, err := scanTask(rows, viewerID, viewerIsAdmin)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// GetTask loads one task; it must involve the viewer unless they are an admin.
func GetTask(db *sql.DB, id, viewerID int, viewerIsAdmin bool) (*Task, error) {
	t, err := scanTask(db.QueryRow(taskSelect+` WHERE t.id = ?`, id), viewerID, viewerIsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	if !viewerIsAdmin && t.CreatedBy != viewerID && t.AssignedTo != viewerID {
		return nil, ErrTaskNotFound
	}
	return t, nil
}

func userExists(db *sql.DB, id int) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, id).Scan(&n)
	return n > 0, err
}

// CreateTask creates a task; with no assignee it is assigned to the creator.
func CreateTask(db *sql.DB, creatorID int, creatorIsAdmin bool, in TaskInput) (*Task, error) {
	if in.Title == nil {
		return nil, &TaskValidationError{"Give the task a title."}
	}
	if err := normalizeTaskInput(&in); err != nil {
		return nil, err
	}
	assignee := creatorID
	if in.AssignedTo != nil && *in.AssignedTo > 0 {
		assignee = *in.AssignedTo
	}
	if ok, err := userExists(db, assignee); err != nil {
		return nil, err
	} else if !ok {
		return nil, &TaskValidationError{"That person was not found."}
	}
	status, priority, desc, due := TaskStatusTodo, TaskPriorityMedium, "", ""
	if in.Status != nil {
		status = *in.Status
	}
	if in.Priority != nil {
		priority = *in.Priority
	}
	if in.Description != nil {
		desc = *in.Description
	}
	if in.DueDate != nil {
		due = *in.DueDate
	}
	var completed interface{}
	if status == TaskStatusDone {
		completed = time.Now()
	}
	res, err := db.Exec(`
		INSERT INTO tasks (title, description, status, priority, due_date, created_by, assigned_to, completed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
		*in.Title, nullIfEmpty(desc), status, priority, nullIfEmpty(due), creatorID, assignee, completed)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetTask(db, int(id), creatorID, creatorIsAdmin)
}

// UpdateTask applies changes. Creators and admins can change everything;
// the assignee can only move the status.
func UpdateTask(db *sql.DB, id, viewerID int, viewerIsAdmin bool, in TaskInput) (*Task, error) {
	current, err := GetTask(db, id, viewerID, viewerIsAdmin)
	if err != nil {
		return nil, err
	}
	if err := normalizeTaskInput(&in); err != nil {
		return nil, err
	}
	onlyStatus := in.Title == nil && in.Description == nil && in.Priority == nil && in.DueDate == nil && in.AssignedTo == nil
	if !current.CanEdit && !(onlyStatus && current.CanChangeStatus) {
		return nil, ErrTaskForbidden
	}

	sets, args := []string{}, []interface{}{}
	if in.Title != nil {
		sets, args = append(sets, "title = ?"), append(args, *in.Title)
	}
	if in.Description != nil {
		sets, args = append(sets, "description = ?"), append(args, nullIfEmpty(*in.Description))
	}
	if in.Priority != nil {
		sets, args = append(sets, "priority = ?"), append(args, *in.Priority)
	}
	if in.DueDate != nil {
		sets, args = append(sets, "due_date = ?"), append(args, nullIfEmpty(*in.DueDate))
	}
	if in.AssignedTo != nil && *in.AssignedTo > 0 && *in.AssignedTo != current.AssignedTo {
		if ok, err := userExists(db, *in.AssignedTo); err != nil {
			return nil, err
		} else if !ok {
			return nil, &TaskValidationError{"That person was not found."}
		}
		sets, args = append(sets, "assigned_to = ?"), append(args, *in.AssignedTo)
	}
	if in.Status != nil && *in.Status != current.Status {
		sets, args = append(sets, "status = ?"), append(args, *in.Status)
		if *in.Status == TaskStatusDone {
			sets = append(sets, "completed_at = NOW()")
		} else {
			sets = append(sets, "completed_at = NULL")
		}
	}
	if len(sets) == 0 {
		return current, nil
	}
	args = append(args, id)
	if _, err := db.Exec(`UPDATE tasks SET `+strings.Join(sets, ", ")+`, updated_at = NOW() WHERE id = ?`, args...); err != nil {
		return nil, err
	}
	return GetTask(db, id, viewerID, viewerIsAdmin)
}

// DeleteTask removes a task; only its creator or an admin may.
func DeleteTask(db *sql.DB, id, viewerID int, viewerIsAdmin bool) error {
	current, err := GetTask(db, id, viewerID, viewerIsAdmin)
	if err != nil {
		return err
	}
	if !current.CanEdit {
		return ErrTaskForbidden
	}
	_, err = db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// SummarizeTasks counts the open work assigned to a user.
func SummarizeTasks(db *sql.DB, userID int) (TaskSummary, error) {
	var s TaskSummary
	today := time.Now().Format("2006-01-02")
	err := db.QueryRow(`
		SELECT
		  COALESCE(SUM(CASE WHEN status <> 'done' THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN status <> 'done' AND due_date < ? THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN status <> 'done' AND due_date = ? THEN 1 ELSE 0 END), 0)
		FROM tasks WHERE assigned_to = ?`, today, today, userID).Scan(&s.Open, &s.Overdue, &s.DueToday)
	return s, err
}
