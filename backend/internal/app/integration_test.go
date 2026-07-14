package app

import (
	"Backend/internal/db"
	"Backend/internal/domain"
	"Backend/internal/domain/backend_entities"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	testApp    *App
	testServer *httptest.Server
	testDB     *gorm.DB
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:15-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "quiz_test",
				"POSTGRES_PASSWORD": "quiz_test",
				"POSTGRES_DB":       "quiz_test",
			},
			WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not start test PostgreSQL container:", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		panic(err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		panic(err)
	}

	dsn := fmt.Sprintf("host=%s port=%s user=quiz_test password=quiz_test dbname=quiz_test sslmode=disable", host, port.Port())
	testDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := testDB.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error; err != nil {
		panic(err)
	}
	if err := testDB.AutoMigrate(
		domain.User{}, domain.Quiz{}, domain.Answer{}, domain.Question{},
		domain.GameSession{}, domain.Participant{}, domain.PlayerAnswer{}, backend_entities.Token{},
	); err != nil {
		panic(err)
	}

	gin.SetMode(gin.TestMode)
	jwtSecret = []byte("integration-test-secret")
	testApp = &App{
		router: gin.New(),
		db:     &db.DB{DB: testDB},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		hub:    NewHub(),
	}
	testApp.registerRoutes()
	testServer = httptest.NewServer(testApp.router)

	code := m.Run()
	testServer.Close()
	if err := container.Terminate(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "could not terminate test PostgreSQL container:", err)
	}
	os.Exit(code)
}

func TestAuthenticationFlow(t *testing.T) {
	email := uniqueEmail()
	response := request(t, http.MethodPost, "/api/user/register", map[string]string{"email": email, "password": "password123"}, "")
	assertStatus(t, response, http.StatusCreated)
	response.Body.Close()

	login := login(t, email)
	response = request(t, http.MethodPost, "/api/user/refresh_token", map[string]string{"refresh_token": login.RefreshToken}, "")
	assertStatus(t, response, http.StatusCreated)
	var refreshed loginResponse
	decodeJSON(t, response, &refreshed)
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" || refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("refresh endpoint did not issue a new token pair")
	}

	response = request(t, http.MethodGet, "/api/quizzes", nil, "")
	assertStatus(t, response, http.StatusUnauthorized)
	response.Body.Close()
}

func TestQuizCRUDAndUpload(t *testing.T) {
	credentials := newOrganizer(t)
	quiz := createQuiz(t, credentials.AccessToken, "CRUD quiz", 10)

	response := request(t, http.MethodGet, "/api/quizzes", nil, credentials.AccessToken)
	assertStatus(t, response, http.StatusOK)
	var quizzes []quizResponse
	decodeJSON(t, response, &quizzes)
	if !containsQuiz(quizzes, quiz.ID) {
		t.Fatalf("quiz %s was not returned to its creator", quiz.ID)
	}

	response = request(t, http.MethodGet, "/api/quizzes/"+quiz.ID, nil, credentials.AccessToken)
	assertStatus(t, response, http.StatusOK)
	var details quizResponse
	decodeJSON(t, response, &details)
	if len(details.Questions) != 1 || len(details.Questions[0].Answers) != 2 {
		t.Fatalf("quiz details did not contain questions and answers: %#v", details)
	}

	uploadPath := uploadImage(t, credentials.AccessToken)
	defer os.Remove(filepath.FromSlash(strings.TrimPrefix(uploadPath, "/")))
	if _, err := os.Stat(filepath.FromSlash(strings.TrimPrefix(uploadPath, "/"))); err != nil {
		t.Fatalf("uploaded file does not exist: %v", err)
	}

	response = request(t, http.MethodDelete, "/api/quizzes/"+quiz.ID, nil, credentials.AccessToken)
	assertStatus(t, response, http.StatusNoContent)
	response.Body.Close()
	response = request(t, http.MethodGet, "/api/quizzes/"+quiz.ID, nil, credentials.AccessToken)
	assertStatus(t, response, http.StatusNotFound)
	response.Body.Close()
}

func TestSessionAndWebSocketGameFlow(t *testing.T) {
	credentials := newOrganizer(t)
	quiz := createQuiz(t, credentials.AccessToken, "WebSocket quiz", 30)
	session := createSession(t, credentials.AccessToken, quiz.ID)

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws/join?pin=" + session.PinCode
	host, response, err := websocket.DefaultDialer.Dial(wsURL+"&name=host", nil)
	if err == nil {
		host.Close()
		t.Fatal("host connection without a token must be rejected")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated host, got %#v", response)
	}

	hostDialer := websocket.Dialer{Subprotocols: []string{"access_token", credentials.AccessToken}}
	host, response, err = hostDialer.Dial(wsURL+"&name=host", nil)
	if err != nil {
		t.Fatalf("connect host: %v", err)
	}
	defer host.Close()

	player, _, err := websocket.DefaultDialer.Dial(wsURL+"&name=Player", nil)
	if err != nil {
		t.Fatalf("connect player: %v", err)
	}
	defer player.Close()

	if err := host.WriteJSON(clientMessage{Type: "start_game"}); err != nil {
		t.Fatalf("start game: %v", err)
	}
	question := readMessageOfType(t, player, "question")
	questionData := question["question"].(map[string]any)
	answers := questionData["answers"].([]any)
	if _, exists := answers[0].(map[string]any)["is_correct"]; exists {
		t.Fatal("question payload must not contain correct-answer flags")
	}
	var correctAnswer domain.Answer
	questionID := questionData["id"].(string)
	if err := testDB.Where("question_id = ? AND is_correct = ?", questionID, true).First(&correctAnswer).Error; err != nil {
		t.Fatalf("load correct test answer: %v", err)
	}

	if err := player.WriteJSON(clientMessage{Type: "submit_answer", AnswersID: []string{correctAnswer.ID.String()}}); err != nil {
		t.Fatalf("submit answer: %v", err)
	}
	waitFor(t, func() bool {
		var count int64
		return testDB.Model(&domain.PlayerAnswer{}).Where("question_id = ?", questionID).Count(&count).Error == nil && count == 1
	})
	if err := host.WriteJSON(clientMessage{Type: "question_time_over"}); err != nil {
		t.Fatalf("finish question: %v", err)
	}
	result := readMessageOfType(t, player, "question_result")
	if len(result["correct_answer_ids"].([]any)) != 1 {
		t.Fatalf("unexpected correct answers: %#v", result)
	}

	var participant domain.Participant
	if err := testDB.Where("session_id = ? AND name = ?", session.ID, "Player").First(&participant).Error; err != nil {
		t.Fatal(err)
	}
	if participant.Score != 1 {
		t.Fatalf("expected player score 1, got %d", participant.Score)
	}

	if err := host.WriteJSON(clientMessage{Type: "next_question"}); err != nil {
		t.Fatalf("next question: %v", err)
	}
	readMessageOfType(t, player, "game_finished")
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type quizResponse struct {
	ID        string `json:"ID"`
	Questions []struct {
		Answers []struct {
			ID string `json:"ID"`
		} `json:"Answers"`
	} `json:"Questions"`
}

type sessionResponse struct {
	ID      string `json:"session_id"`
	PinCode string `json:"pin_code"`
}

func newOrganizer(t *testing.T) loginResponse {
	t.Helper()
	email := uniqueEmail()
	response := request(t, http.MethodPost, "/api/user/register", map[string]string{"email": email, "password": "password123"}, "")
	assertStatus(t, response, http.StatusCreated)
	response.Body.Close()
	return login(t, email)
}

func login(t *testing.T, email string) loginResponse {
	t.Helper()
	response := request(t, http.MethodPost, "/api/user/login", map[string]string{"email": email, "password": "password123"}, "")
	assertStatus(t, response, http.StatusOK)
	var result loginResponse
	decodeJSON(t, response, &result)
	return result
}

func createQuiz(t *testing.T, token, title string, duration int) quizResponse {
	t.Helper()
	payload := map[string]any{
		"title":             title,
		"question_duration": duration,
		"questions": []any{map[string]any{
			"content": "What is two plus two?",
			"type":    "single",
			"answers": []any{
				map[string]any{"content": "4", "is_correct": true},
				map[string]any{"content": "5", "is_correct": false},
			},
		}},
	}
	response := request(t, http.MethodPost, "/api/quizzes", payload, token)
	assertStatus(t, response, http.StatusCreated)
	var quiz quizResponse
	decodeJSON(t, response, &quiz)
	if quiz.ID == "" {
		t.Fatal("created quiz has no id")
	}
	return quiz
}

func createSession(t *testing.T, token, quizID string) sessionResponse {
	t.Helper()
	response := request(t, http.MethodPost, "/api/sessions", map[string]string{"quiz_id": quizID}, token)
	assertStatus(t, response, http.StatusCreated)
	var session sessionResponse
	decodeJSON(t, response, &session)
	if len(session.PinCode) != 6 {
		t.Fatalf("expected six digit PIN, got %q", session.PinCode)
	}
	return session
}

func uploadImage(t *testing.T, token string) string {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "image.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("not a real image, extension validation is sufficient for this endpoint"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/api/upload", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected 201, got %d: %s", response.StatusCode, body)
	}
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload.Path
}

func request(t *testing.T, method, path string, payload any, token string) *http.Response {
	t.Helper()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, testServer.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func assertStatus(t *testing.T, response *http.Response, expected int) {
	t.Helper()
	if response.StatusCode != expected {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("expected HTTP %d, got %d: %s", expected, response.StatusCode, body)
	}
}

func decodeJSON(t *testing.T, response *http.Response, value any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(value); err != nil {
		t.Fatal(err)
	}
}

func readMessageOfType(t *testing.T, connection *websocket.Conn, expectedType string) map[string]any {
	t.Helper()
	if err := connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		var message map[string]any
		if err := connection.ReadJSON(&message); err != nil {
			t.Fatalf("read %s message: %v", expectedType, err)
		}
		if message["type"] == expectedType {
			return message
		}
	}
}

func uniqueEmail() string {
	return "organizer-" + uuid.NewString() + "@example.test"
}

func containsQuiz(quizzes []quizResponse, id string) bool {
	for _, quiz := range quizzes {
		if quiz.ID == id {
			return true
		}
	}
	return false
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied before timeout")
}
