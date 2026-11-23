package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// BirthdayPerson is one colleague on the shared birthday board.
type BirthdayPerson struct {
	ID            int    `json:"id"`
	Username      string `json:"username"`
	BirthdayLabel string `json:"birthday_label"`
	IsToday       bool   `json:"is_today"`
	ImageURL      string `json:"image_url,omitempty"`
	Wish          string `json:"wish,omitempty"`
	DaysUntil     int    `json:"days_until"`
}

// ParseBirthDate accepts YYYY-MM-DD and rejects empty, future, or implausible dates.
func ParseBirthDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("birth date is required")
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("birth date must be YYYY-MM-DD")
	}
	today := birthdayToday()
	if parsed.Year() < 1900 || parsed.After(today) {
		return time.Time{}, fmt.Errorf("enter a real birth date")
	}
	return parsed, nil
}

// BirthdayWish is a short paperless greeting for the birthday itself.
func BirthdayWish(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "friend"
	}
	return fmt.Sprintf("Happy birthday, %s. Wishing you a gentle year ahead, grown with care and nothing wasted.", name)
}

func birthdayToday() time.Time {
	loc, err := time.LoadLocation("Africa/Johannesburg")
	if err != nil {
		return time.Now()
	}
	return time.Now().In(loc)
}

// IsBirthdayToday matches month and day. A 29 February birthday is marked on 28 February in other years.
func IsBirthdayToday(birth, today time.Time) bool {
	month, day := birthdayMonthDay(birth, today.Year())
	return today.Month() == month && today.Day() == day
}

func birthdayMonthDay(birth time.Time, year int) (time.Month, int) {
	if birth.Month() == time.February && birth.Day() == 29 && !isLeapYear(year) {
		return time.February, 28
	}
	return birth.Month(), birth.Day()
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func daysUntilBirthday(birth, today time.Time) int {
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	month, day := birthdayMonthDay(birth, today.Year())
	next := time.Date(today.Year(), month, day, 0, 0, 0, 0, today.Location())
	if next.Before(todayDate) {
		month, day = birthdayMonthDay(birth, today.Year()+1)
		next = time.Date(today.Year()+1, month, day, 0, 0, 0, 0, today.Location())
	}
	return int(next.Sub(todayDate).Hours() / 24)
}

// ListBirthdays returns every saved birth date, soonest first, with a wish and photo on the day.
func ListBirthdays(db *sql.DB) ([]BirthdayPerson, error) {
	rows, err := db.Query(`
		SELECT u.id, u.username, u.birth_date, COALESCE(f.image_url, '')
		FROM users u
		LEFT JOIN user_faces f ON f.user_id = u.id
		WHERE u.birth_date IS NOT NULL
		ORDER BY u.username ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := birthdayToday()
	people := make([]BirthdayPerson, 0)
	for rows.Next() {
		var (
			person BirthdayPerson
			birth  time.Time
		)
		if err := rows.Scan(&person.ID, &person.Username, &birth, &person.ImageURL); err != nil {
			return nil, err
		}
		person.ImageURL = strings.TrimSpace(person.ImageURL)
		month, day := birthdayMonthDay(birth, today.Year())
		person.BirthdayLabel = fmt.Sprintf("%d %s", day, month.String())
		person.IsToday = IsBirthdayToday(birth, today)
		person.DaysUntil = daysUntilBirthday(birth, today)
		if person.IsToday {
			person.Wish = BirthdayWish(person.Username)
		}
		people = append(people, person)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortBirthdays(people)
	return people, nil
}

func sortBirthdays(people []BirthdayPerson) {
	for i := 1; i < len(people); i++ {
		item := people[i]
		j := i
		for j > 0 && birthdayBefore(item, people[j-1]) {
			people[j] = people[j-1]
			j--
		}
		people[j] = item
	}
}

func birthdayBefore(a, b BirthdayPerson) bool {
	if a.DaysUntil != b.DaysUntil {
		return a.DaysUntil < b.DaysUntil
	}
	return strings.ToLower(a.Username) < strings.ToLower(b.Username)
}
