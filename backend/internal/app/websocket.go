package app

import (
	"Backend/internal/domain"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

type Hub struct {
	mu         sync.RWMutex
	roomsByPIN map[string]*Room
	roomsByID  map[uuid.UUID]*Room
}

type Room struct {
	SessionID   uuid.UUID
	PinCode     string
	connections map[*websocket.Conn]*domain.Participant
	hosts       map[*websocket.Conn]bool
	broadcast   chan []byte

	mu                   sync.RWMutex
	currentQuestionIndex int
	timerActive          bool
	status               string
	questionDuration     time.Duration
	questions            []domain.Question
}

type clientMessage struct {
	Type      string   `json:"type"`
	AnswersID []string `json:"answer_ids"`
}

type playerView struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Score int       `json:"score"`
}

type answerView struct {
	ID      uuid.UUID `json:"id"`
	Content string    `json:"content"`
}

type questionView struct {
	ID        uuid.UUID    `json:"id"`
	Type      string       `json:"type"`
	Content   string       `json:"content"`
	ImagePath string       `json:"image_path,omitempty"`
	Answers   []answerView `json:"answers"`
}

func NewHub() *Hub {
	return &Hub{roomsByPIN: make(map[string]*Room), roomsByID: make(map[uuid.UUID]*Room)}
}

func (h *Hub) CreateRoom(session domain.GameSession) *Room {
	room := &Room{
		SessionID:            session.ID,
		PinCode:              session.PinCode,
		connections:          make(map[*websocket.Conn]*domain.Participant),
		hosts:                make(map[*websocket.Conn]bool),
		broadcast:            make(chan []byte, 64),
		currentQuestionIndex: -1,
		status:               session.Status,
	}
	h.mu.Lock()
	h.roomsByPIN[session.PinCode] = room
	h.roomsByID[session.ID] = room
	h.mu.Unlock()
	go room.run()
	return room
}

func (h *Hub) RoomByPIN(pin string) *Room {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.roomsByPIN[pin]
}

func (r *Room) run() {
	for message := range r.broadcast {
		r.mu.RLock()
		connections := make([]*websocket.Conn, 0, len(r.connections))
		for connection := range r.connections {
			connections = append(connections, connection)
		}
		r.mu.RUnlock()

		for _, connection := range connections {
			if err := connection.WriteMessage(websocket.TextMessage, message); err != nil {
				r.removeConnection(connection)
				_ = connection.Close()
			}
		}
	}
}

func (r *Room) Broadcast(payload any) {
	message, err := json.Marshal(payload)
	if err != nil {
		return
	}
	r.broadcast <- message
}

func (r *Room) addConnection(connection *websocket.Conn, participant *domain.Participant, isHost bool) {
	r.mu.Lock()
	r.connections[connection] = participant
	r.hosts[connection] = isHost
	r.mu.Unlock()
}

func (r *Room) removeConnection(connection *websocket.Conn) {
	r.mu.Lock()
	delete(r.connections, connection)
	delete(r.hosts, connection)
	r.mu.Unlock()
}

func (r *Room) activePlayers() []playerView {
	r.mu.RLock()
	defer r.mu.RUnlock()
	players := make([]playerView, 0, len(r.connections))
	for _, participant := range r.connections {
		if participant != nil {
			players = append(players, playerView{ID: participant.ID, Name: participant.Name, Score: participant.Score})
		}
	}
	sort.Slice(players, func(i, j int) bool { return players[i].Name < players[j].Name })
	return players
}

func (a *App) handleJoinWebSocket(c *gin.Context) {
	pin := c.Query("pin")
	name := strings.TrimSpace(c.Query("name"))
	if pin == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pin and name are required"})
		return
	}

	room := a.hub.RoomByPIN(pin)
	if room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "game room not found"})
		return
	}

	isHost := strings.EqualFold(name, "host")
	if isHost {
		userID, err := accessTokenUserID(accessTokenFromWebSocketProtocols(c.Request))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "a valid access token is required for the host"})
			return
		}

		var session domain.GameSession
		if err := a.db.Preload("Quiz").First(&session, room.SessionID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "game session not found"})
			return
		}
		if session.Quiz.CreatorID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "only the quiz creator can host this session"})
			return
		}
	}

	connection, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		a.logger.Error("failed to upgrade websocket", "error", err)
		return
	}

	var participant *domain.Participant
	if !isHost {
		newParticipant := domain.Participant{SessionID: room.SessionID, Name: name}
		if err := a.db.Create(&newParticipant).Error; err != nil {
			_ = connection.Close()
			a.logger.Error("failed to create participant", "error", err)
			return
		}
		participant = &newParticipant
		if err := connection.WriteJSON(gin.H{"type": "joined", "participant": playerView{ID: participant.ID, Name: participant.Name, Score: participant.Score}}); err != nil {
			_ = connection.Close()
			return
		}
	}

	room.addConnection(connection, participant, isHost)
	a.broadcastLobby(room)

	defer func() {
		room.removeConnection(connection)
		_ = connection.Close()
		a.broadcastLobby(room)
	}()

	for {
		var message clientMessage
		if err := connection.ReadJSON(&message); err != nil {
			return
		}

		switch message.Type {
		case "start_game":
			if isHost {
				a.startGame(room)
			}
		case "submit_answer":
			if participant != nil {
				a.submitAnswer(room, *participant, message.AnswersID)
			}
		case "question_time_over":
			if isHost {
				a.finishQuestion(room)
			}
		case "next_question":
			if isHost {
				a.nextQuestion(room)
			}
		}
	}
}

func accessTokenFromWebSocketProtocols(request *http.Request) string {
	protocols := websocket.Subprotocols(request)
	for i := 0; i+1 < len(protocols); i++ {
		if protocols[i] == "access_token" {
			return protocols[i+1]
		}
	}
	return ""
}

func (a *App) broadcastLobby(room *Room) {
	room.Broadcast(gin.H{"type": "lobby_update", "players": room.activePlayers()})
}

func (a *App) startGame(room *Room) {
	room.mu.Lock()
	if room.status != "waiting" {
		room.mu.Unlock()
		return
	}
	room.status = "starting"
	room.mu.Unlock()

	var session domain.GameSession
	if err := a.db.Preload("Quiz.Questions.Answers").First(&session, room.SessionID).Error; err != nil {
		room.mu.Lock()
		room.status = "waiting"
		room.mu.Unlock()
		a.logger.Error("failed to load session quiz", "error", err, "session_id", room.SessionID)
		return
	}
	if len(session.Quiz.Questions) == 0 {
		room.mu.Lock()
		room.status = "waiting"
		room.mu.Unlock()
		return
	}

	room.mu.Lock()
	room.questions = session.Quiz.Questions
	room.questionDuration = time.Duration(session.Quiz.QuestionDuration) * time.Second
	room.status = "playing"
	room.currentQuestionIndex = 0
	room.timerActive = true
	room.mu.Unlock()

	if err := a.db.Model(&domain.GameSession{}).Where("id = ?", room.SessionID).Update("status", "playing").Error; err != nil {
		a.logger.Error("failed to start session", "error", err, "session_id", room.SessionID)
	}
	a.broadcastQuestion(room)
	a.startQuestionTimer(room, 0)
}

func (a *App) startQuestionTimer(room *Room, questionIndex int) {
	room.mu.RLock()
	duration := room.questionDuration
	room.mu.RUnlock()
	go func() {
		time.Sleep(duration)
		room.mu.RLock()
		shouldFinish := room.status == "playing" && room.timerActive && room.currentQuestionIndex == questionIndex
		room.mu.RUnlock()
		if shouldFinish {
			a.finishQuestion(room)
		}
	}()
}

func (a *App) broadcastQuestion(room *Room) {
	room.mu.RLock()
	if room.currentQuestionIndex < 0 || room.currentQuestionIndex >= len(room.questions) {
		room.mu.RUnlock()
		return
	}
	question := room.questions[room.currentQuestionIndex]
	durationSeconds := int(room.questionDuration.Seconds())
	room.mu.RUnlock()

	answers := make([]answerView, 0, len(question.Answers))
	for _, answer := range question.Answers {
		answers = append(answers, answerView{ID: answer.ID, Content: answer.Content})
	}
	room.Broadcast(gin.H{"type": "question", "question": questionView{
		ID: question.ID, Type: question.Type, Content: question.Content, ImagePath: question.ImagePath, Answers: answers,
	}, "duration_seconds": durationSeconds})
}

func (a *App) submitAnswer(room *Room, participant domain.Participant, answerIDs []string) {
	room.mu.RLock()
	if room.status != "playing" || !room.timerActive || room.currentQuestionIndex < 0 || room.currentQuestionIndex >= len(room.questions) {
		room.mu.RUnlock()
		return
	}
	question := room.questions[room.currentQuestionIndex]
	room.mu.RUnlock()

	validAnswerIDs := make(map[uuid.UUID]bool, len(question.Answers))
	for _, answer := range question.Answers {
		validAnswerIDs[answer.ID] = true
	}

	answers := make([]domain.PlayerAnswer, 0, len(answerIDs))
	seen := make(map[uuid.UUID]bool, len(answerIDs))
	for _, rawID := range answerIDs {
		answerID, err := uuid.Parse(rawID)
		if err != nil || !validAnswerIDs[answerID] || seen[answerID] {
			return
		}
		seen[answerID] = true
		answers = append(answers, domain.PlayerAnswer{ParticipantID: participant.ID, QuestionID: question.ID, AnswerID: answerID})
	}

	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("participant_id = ? AND question_id = ?", participant.ID, question.ID).Delete(&domain.PlayerAnswer{}).Error; err != nil {
			return err
		}
		if len(answers) > 0 {
			return tx.Create(&answers).Error
		}
		return nil
	}); err != nil {
		a.logger.Error("failed to save player answers", "error", err, "participant_id", participant.ID)
	}
}

func (a *App) finishQuestion(room *Room) {
	room.mu.Lock()
	if room.status != "playing" || !room.timerActive || room.currentQuestionIndex < 0 || room.currentQuestionIndex >= len(room.questions) {
		room.mu.Unlock()
		return
	}
	question := room.questions[room.currentQuestionIndex]
	room.timerActive = false
	room.mu.Unlock()

	correctAnswerIDs := make(map[uuid.UUID]bool)
	correctAnswerIDList := make([]uuid.UUID, 0)
	answerCounts := make(map[uuid.UUID]int, len(question.Answers))
	for _, answer := range question.Answers {
		answerCounts[answer.ID] = 0
	}
	for _, answer := range question.Answers {
		if answer.IsCorrect {
			correctAnswerIDs[answer.ID] = true
			correctAnswerIDList = append(correctAnswerIDList, answer.ID)
		}
	}

	err := a.db.Transaction(func(tx *gorm.DB) error {
		var participants []domain.Participant
		if err := tx.Where("session_id = ?", room.SessionID).Find(&participants).Error; err != nil {
			return err
		}
		for _, participant := range participants {
			var submitted []domain.PlayerAnswer
			if err := tx.Where("participant_id = ? AND question_id = ?", participant.ID, question.ID).Find(&submitted).Error; err != nil {
				return err
			}
			selected := make(map[uuid.UUID]bool, len(submitted))
			for _, answer := range submitted {
				selected[answer.AnswerID] = true
				answerCounts[answer.AnswerID]++
			}
			if len(selected) == len(correctAnswerIDs) {
				isCorrect := true
				for answerID := range correctAnswerIDs {
					if !selected[answerID] {
						isCorrect = false
						break
					}
				}
				if isCorrect {
					if err := tx.Model(&domain.Participant{}).Where("id = ?", participant.ID).Update("score", gorm.Expr("score + ?", 1)).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		a.logger.Error("failed to score question", "error", err, "session_id", room.SessionID)
		return
	}

	leaderboard, err := a.leaderboard(room.SessionID)
	if err != nil {
		a.logger.Error("failed to load leaderboard", "error", err, "session_id", room.SessionID)
		return
	}
	room.Broadcast(gin.H{"type": "question_result", "correct_answer_ids": correctAnswerIDList, "answer_counts": answerCounts, "leaderboard": leaderboard})
}

func (a *App) nextQuestion(room *Room) {
	room.mu.Lock()
	if room.status != "playing" || room.timerActive {
		room.mu.Unlock()
		return
	}
	nextIndex := room.currentQuestionIndex + 1
	if nextIndex >= len(room.questions) {
		room.status = "finished"
		room.mu.Unlock()
		if err := a.db.Model(&domain.GameSession{}).Where("id = ?", room.SessionID).Update("status", "finished").Error; err != nil {
			a.logger.Error("failed to finish session", "error", err, "session_id", room.SessionID)
		}
		room.Broadcast(gin.H{"type": "game_finished"})
		return
	}
	room.currentQuestionIndex = nextIndex
	room.timerActive = true
	room.mu.Unlock()

	a.broadcastQuestion(room)
	a.startQuestionTimer(room, nextIndex)
}

func (a *App) leaderboard(sessionID uuid.UUID) ([]playerView, error) {
	var participants []domain.Participant
	if err := a.db.Where("session_id = ?", sessionID).Order("score DESC, name ASC").Find(&participants).Error; err != nil {
		return nil, err
	}
	leaderboard := make([]playerView, 0, len(participants))
	for _, participant := range participants {
		leaderboard = append(leaderboard, playerView{ID: participant.ID, Name: participant.Name, Score: participant.Score})
	}
	return leaderboard, nil
}
