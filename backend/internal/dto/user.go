package dto

type UserRegistrationRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type UserLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type UserRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type CreateQuizRequest struct {
	Title            string           `json:"title" binding:"required"`
	QuestionDuration int              `json:"question_duration" binding:"required,gt=0"`
	Questions        []CreateQuestion `json:"questions" binding:"required,min=1"`
}

type CreateQuestion struct {
	Content   string         `json:"content" binding:"required"`
	Type      string         `json:"type" binding:"required,oneof=single multiple"`
	ImagePath string         `json:"image_path"`
	Answers   []CreateAnswer `json:"answers" binding:"required,min=1"`
}

type CreateAnswer struct {
	Content   string `json:"content" binding:"required"`
	IsCorrect bool   `json:"is_correct"`
}
