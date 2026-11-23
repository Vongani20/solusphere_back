package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"solusphere_backend/models"
)

const (
	cvImportJobTTL      = 20 * time.Minute
	cvImportJobTimeout  = 3 * time.Minute
	cvImportJobStatusOK = "done"
	cvImportJobWorking  = "processing"
	cvImportJobFailed   = "error"
)

// CVImportJob is an in-memory import that outlives the HTTP request so
// CloudFront's 60s origin timeout cannot cancel OpenAI OCR.
type CVImportJob struct {
	ID        string
	UserID    int
	Status    string
	Profile   *models.CVProfile
	Warnings  []string
	Error     string
	CreatedAt time.Time
}

var (
	cvImportJobs   = map[string]*CVImportJob{}
	cvImportJobsMu sync.Mutex
)

func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			pruneCVImportJobs()
		}
	}()
}

func pruneCVImportJobs() {
	cutoff := time.Now().Add(-cvImportJobTTL)
	cvImportJobsMu.Lock()
	defer cvImportJobsMu.Unlock()
	for id, job := range cvImportJobs {
		if job.CreatedAt.Before(cutoff) {
			delete(cvImportJobs, id)
		}
	}
}

func newCVImportJobID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return hex.EncodeToString(b[:])
}

// StartCVImportJob queues OCR/parse on a background deadline independent of
// the inbound HTTP request.
func StartCVImportJob(userID int, filename string, data []byte) *CVImportJob {
	job := &CVImportJob{
		ID:        newCVImportJobID(),
		UserID:    userID,
		Status:    cvImportJobWorking,
		CreatedAt: time.Now(),
	}
	cvImportJobsMu.Lock()
	cvImportJobs[job.ID] = job
	cvImportJobsMu.Unlock()

	go runCVImportJob(job.ID, filename, append([]byte(nil), data...))
	return job
}

func runCVImportJob(id, filename string, data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), cvImportJobTimeout)
	defer cancel()

	profile, warnings, err := ImportCVProfileFromUpload(ctx, filename, data)

	cvImportJobsMu.Lock()
	defer cvImportJobsMu.Unlock()
	job, ok := cvImportJobs[id]
	if !ok {
		return
	}
	if err != nil {
		log.Printf("CV import job %s failed for %q: %v", id, filename, err)
		job.Status = cvImportJobFailed
		job.Error = err.Error()
		return
	}
	job.Status = cvImportJobStatusOK
	job.Profile = profile
	job.Warnings = warnings
}

// GetCVImportJob returns a job owned by userID, or nil.
func GetCVImportJob(id string, userID int) *CVImportJob {
	cvImportJobsMu.Lock()
	defer cvImportJobsMu.Unlock()
	job, ok := cvImportJobs[id]
	if !ok || job.UserID != userID {
		return nil
	}
	copyJob := *job
	return &copyJob
}
