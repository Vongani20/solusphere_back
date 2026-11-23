package services

import "testing"

func TestStartCVImportJobStoresProcessingJob(t *testing.T) {
	job := StartCVImportJob(42, "empty.bin", []byte{})
	if job == nil || job.ID == "" {
		t.Fatal("expected job id")
	}
	got := GetCVImportJob(job.ID, 42)
	if got == nil {
		t.Fatal("expected stored job")
	}
	if got.UserID != 42 {
		t.Fatalf("user = %d", got.UserID)
	}
	if GetCVImportJob(job.ID, 99) != nil {
		t.Fatal("other users must not see the job")
	}
}
