package models

type FaceLoginResponse struct {
	Message    string      `json:"message"`
	Token      string      `json:"token"`
	Similarity float32     `json:"similarity"`
	User       UserMinimal `json:"user"`
}

type UserMinimal struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	ImageURL string `json:"image_url"`
}
