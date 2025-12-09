package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	username string
	email    string
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long:  `Register, login, and logout commands for MangaHub authentication.`,
}

var authRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new account",
	Long:  `Register a new MangaHub account with username and email.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if username == "" {
			return fmt.Errorf("username is required (--username)")
		}
		if email == "" {
			return fmt.Errorf("email is required (--email)")
		}

		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		password := string(passwordBytes)

		fmt.Print("Confirm password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password confirmation: %w", err)
		}
		confirmPassword := string(confirmBytes)

		if password != confirmPassword {
			printError("Passwords do not match")
			return fmt.Errorf("passwords do not match")
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		reqBody := map[string]string{
			"username": username,
			"email":    email,
			"password": password,
		}
		jsonData, _ := json.Marshal(reqBody)

		res, err := http.Post(serverURL+"/auth/register", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			printError("Registration failed: Server connection error")
			fmt.Println("Check server status: mangahub server status")
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)

		if res.StatusCode != http.StatusCreated {
			var errRes map[string]string
			json.Unmarshal(body, &errRes)

			if strings.Contains(errRes["error"], "already exists") {
				printError(fmt.Sprintf("Registration failed: %s", errRes["error"]))
				fmt.Printf("Try: mangahub auth login --username %s\n", username)
			} else if strings.Contains(errRes["error"], "Invalid email") {
				printError("Registration failed: Invalid email format")
				fmt.Println("Please provide a valid email address")
			} else if strings.Contains(errRes["error"], "weak") || strings.Contains(errRes["error"], "Password") {
				printError("Registration failed: Password too weak")
				fmt.Println("Password must be at least 8 characters with mixed case and numbers")
			} else {
				printError(fmt.Sprintf("Registration failed: %s", errRes["error"]))
			}
			return fmt.Errorf("registration failed")
		}

		var authRes struct {
			Token     string    `json:"token"`
			UserID    string    `json:"user_id"`
			Username  string    `json:"username"`
			Email     string    `json:"email"`
			CreatedAt time.Time `json:"created_at"`
		}
		json.Unmarshal(body, &authRes)

		if err := config.UpdateUserToken(authRes.Username, authRes.Token); err != nil {
			fmt.Println("Warning: Failed to save token to config")
		}

		printSuccess("Account created successfully!")
		fmt.Printf("User ID: %s\n", authRes.UserID)
		fmt.Printf("Username: %s\n", authRes.Username)
		fmt.Printf("Email: %s\n", authRes.Email)
		fmt.Printf("Created: %s\n", authRes.CreatedAt.Format("2006-01-02 15:04:05 MST"))
		fmt.Println("\nYou are now logged in!")
		fmt.Println("Try: mangahub manga search \"your favorite manga\"")

		return nil
	},
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to your account",
	Long:  `Login to your MangaHub account with username or email.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if username == "" && email == "" {
			return fmt.Errorf("username or email is required (--username or --email)")
		}

		//Get password securely
		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		password := string(passwordBytes)

		//Call login API
		serverURL, err := config.GetServerURL()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		reqBody := map[string]string{
			"password": password,
		}
		if username != "" {
			reqBody["username"] = username
		}
		if email != "" {
			reqBody["email"] = email
		}
		jsonData, _ := json.Marshal(reqBody)

		resp, err := http.Post(serverURL+"/auth/login", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			printError("Login failed: Server connection error")
			fmt.Println("Check server status: mangahub server status")
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			var errResp map[string]string
			json.Unmarshal(body, &errResp)

			if strings.Contains(errResp["error"], "Invalid credentials") {
				printError("Login failed: Invalid credentials")
				fmt.Println("Check your username and password")
			} else if strings.Contains(errResp["error"], "not found") {
				printError("Login failed: Account not found")
				identifier := username
				if identifier == "" {
					identifier = email
				}
				fmt.Printf("Try: mangahub auth register --username %s --email %s\n", identifier, email)
			} else {
				printError(fmt.Sprintf("Login failed: %s", errResp["error"]))
			}
			return fmt.Errorf("login failed")
		}

		var authResp struct {
			Token     string    `json:"token"`
			UserID    string    `json:"user_id"`
			Username  string    `json:"username"`
			Email     string    `json:"email"`
			ExpiresAt time.Time `json:"expires_at"`
		}
		json.Unmarshal(body, &authResp)

		//Save token to config
		if err := config.UpdateUserToken(authResp.Username, authResp.Token); err != nil {
			fmt.Println("Warning: Failed to save token to config")
		}

		printSuccess("Login successful!")
		fmt.Printf("Welcome back, %s!\n", authResp.Username)
		fmt.Println("\nSession Details:")
		fmt.Printf("  Token expires: %s (24 hours)\n", authResp.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
		fmt.Println("  Permissions: read, write, sync")

		cfg, _ := config.Load()
		fmt.Printf("  Auto-sync: %v\n", cfg.Sync.AutoSync)
		fmt.Printf("  Notifications: %v\n", cfg.Notifications.Enabled)

		fmt.Println("\nReady to use MangaHub! Try:")
		fmt.Println("  mangahub manga search \"your favorite manga\"")

		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from your account",
	Long:  `Logout from your MangaHub account and remove stored token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not found")
			fmt.Println("Run: mangahub init")
			return err
		}

		if cfg.User.Token == "" {
			printInfo("You are not logged in")
			return nil
		}

		currentUser := cfg.User.Username

		if err := config.ClearUserToken(); err != nil {
			return fmt.Errorf("failed to logout: %w", err)
		}

		printSuccess("Logged out successfully!")
		fmt.Printf("Goodbye, %s!\n", currentUser)
		fmt.Println("\nTo login again:")
		fmt.Printf("  mangahub auth login --username %s\n", currentUser)

		return nil
	},
}

var authChangePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Change your account password",
	Long:  `Change your MangaHub account password with verification of current password.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		if cfg.User.Token == "" {
			printError("You are not logged in")
			return fmt.Errorf("please login first: mangahub auth login")
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			return err
		}

		fmt.Print("Current password: ")
		currentPwdBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}

		fmt.Print("New password: ")
		newPwdBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}

		fmt.Print("Confirm new password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password confirmation: %w", err)
		}

		if string(newPwdBytes) != string(confirmBytes) {
			printError("Passwords do not match")
			return fmt.Errorf("new passwords do not match")
		}

		reqBody := map[string]string{
			"current_password": string(currentPwdBytes),
			"new_password":     string(newPwdBytes),
		}
		jsonData, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("POST", serverURL+"/auth/change-password", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			printError("Failed to change password: Server connection error")
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)

		if res.StatusCode != http.StatusOK {
			var errRes map[string]interface{}
			json.Unmarshal(body, &errRes)
			printError(fmt.Sprintf("Failed to change password: %v", errRes["error"]))
			if details, ok := errRes["details"].(string); ok {
				fmt.Printf("Details: %s\n", details)
			}
			return fmt.Errorf("password change failed")
		}

		printSuccess("Password changed successfully!")
		return nil
	},
}

var authUpdateEmailCmd = &cobra.Command{
	Use:   "update-email",
	Short: "Update your email address",
	Long:  `Update your MangaHub account email address (JWT token required).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		if cfg.User.Token == "" {
			printError("You are not logged in")
			return fmt.Errorf("please login first: mangahub auth login")
		}

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("New email: ")
		newEmail, _ := reader.ReadString('\n')
		newEmail = strings.TrimSpace(newEmail)

		if newEmail == "" {
			printError("Email cannot be empty")
			return fmt.Errorf("email is required")
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			return err
		}

		reqBody := map[string]string{
			"new_email": newEmail,
		}
		jsonData, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("POST", serverURL+"/auth/update-email", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			printError("Failed to update email: Server connection error")
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)

		if res.StatusCode != http.StatusOK {
			var errRes map[string]interface{}
			json.Unmarshal(body, &errRes)
			printError(fmt.Sprintf("Failed to update email: %v", errRes["error"]))
			if details, ok := errRes["details"].(string); ok {
				fmt.Printf("Details: %s\n", details)
			}
			return fmt.Errorf("email update failed")
		}

		printSuccess("Email updated successfully!")
		fmt.Printf("New email: %s\n", newEmail)
		return nil
	},
}

var authUpdateUsernameCmd = &cobra.Command{
	Use:   "update-username",
	Short: "Update your username",
	Long:  `Update your MangaHub account username (JWT token required).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		if cfg.User.Token == "" {
			printError("You are not logged in")
			return fmt.Errorf("please login first: mangahub auth login")
		}

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("New username: ")
		newUsername, _ := reader.ReadString('\n')
		newUsername = strings.TrimSpace(newUsername)

		if newUsername == "" {
			printError("Username cannot be empty")
			return fmt.Errorf("username is required")
		}

		if len(newUsername) < 3 {
			printError("Username must be at least 3 characters")
			return fmt.Errorf("username too short")
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			return err
		}

		reqBody := map[string]string{
			"new_username": newUsername,
		}
		jsonData, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("POST", serverURL+"/auth/update-username", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			printError("Failed to update username: Server connection error")
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)

		if res.StatusCode != http.StatusOK {
			var errRes map[string]interface{}
			json.Unmarshal(body, &errRes)
			printError(fmt.Sprintf("Failed to update username: %v", errRes["error"]))
			if details, ok := errRes["details"].(string); ok {
				fmt.Printf("Details: %s\n", details)
			}
			return fmt.Errorf("username update failed")
		}

		printSuccess("Username updated successfully!")
		fmt.Printf("New username: %s\n", newUsername)
		if err := config.UpdateUserToken(newUsername, cfg.User.Token); err != nil {
			fmt.Println("Warning: Could not update local config with new username")
		}
		return nil
	},
}

func init() {
	authRegisterCmd.Flags().StringVar(&username, "username", "", "Username for registration")
	authRegisterCmd.Flags().StringVar(&email, "email", "", "Email for registration")
	authRegisterCmd.MarkFlagRequired("username")
	authRegisterCmd.MarkFlagRequired("email")

	authLoginCmd.Flags().StringVar(&username, "username", "", "Username for login")
	authLoginCmd.Flags().StringVar(&email, "email", "", "Email for login")

	authCmd.AddCommand(authRegisterCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authChangePasswordCmd)
	authCmd.AddCommand(authUpdateEmailCmd)
	authCmd.AddCommand(authUpdateUsernameCmd)
}

func readPasswordFallback() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(password), nil
}
