package models

import "time"

type User struct {
	ID           string    `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         string    `json:"role" db:"role"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"omitempty,max=255"` // Either username or email is required
	Email    string `json:"email" binding:"omitempty,email"`      // Either username or email is required
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdateEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
}

type UpdateUsernameRequest struct {
	NewUsername string `json:"new_username" binding:"required,min=3,max=50"`
}

type UpdateProfileResponse struct {
	Message   string    `json:"message"`
	UserID    string    `json:"user_id"`
	Email     string    `json:"email,omitempty"`
	Username  string    `json:"username,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}
