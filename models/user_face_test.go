package models

import "testing"

func TestUserFaceIsRegistered(t *testing.T) {
	tests := []struct {
		name string
		face *UserFace
		want bool
	}{
		{name: "nil face", face: nil, want: false},
		{name: "inactive face", face: &UserFace{Status: false, FaceID: "face-1"}, want: false},
		{name: "active without face id", face: &UserFace{Status: true}, want: false},
		{name: "active with face id", face: &UserFace{Status: true, FaceID: "face-1"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.face.IsRegistered(); got != tt.want {
				t.Fatalf("IsRegistered() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestS3ObjectURLRoundTrip(t *testing.T) {
	oldBucket := BucketName
	oldRegion := AWSRegionName
	defer func() {
		BucketName = oldBucket
		AWSRegionName = oldRegion
	}()

	BucketName = "example-bucket"
	AWSRegionName = "eu-west-1"

	key := "faces/user_12/face 12.jpg"
	rawURL := S3ObjectURL(key)
	got, ok := S3KeyFromObjectURL(rawURL)
	if !ok {
		t.Fatalf("expected URL to parse as S3 object URL: %s", rawURL)
	}
	if got != key {
		t.Fatalf("S3KeyFromObjectURL() = %q, want %q", got, key)
	}
}
