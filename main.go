package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Note — модель заметки
type Note struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Store — хранилище заметок в памяти (потокобезопасное)
type Store struct {
	mu     sync.Mutex
	notes  map[int]Note
	nextID int
}

func NewStore() *Store {
	return &Store{
		notes:  make(map[int]Note),
		nextID: 1,
	}
}

func (s *Store) Create(title, body string) Note {
	s.mu.Lock()
	defer s.mu.Unlock()

	note := Note{
		ID:        s.nextID,
		Title:     title,
		Body:      body,
		CreatedAt: time.Now(),
	}
	s.notes[s.nextID] = note
	s.nextID++
	return note
}

func (s *Store) List() []Note {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		result = append(result, n)
	}
	return result
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.notes[id]; !ok {
		return false
	}
	delete(s.notes, id)
	return true
}

// --- HTTP-обработчики ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *Store) handleNotes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.List())

	case http.MethodPost:
		var input struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, `{"error":"некорректный JSON"}`, http.StatusBadRequest)
			return
		}
		if input.Title == "" {
			http.Error(w, `{"error":"title обязателен"}`, http.StatusBadRequest)
			return
		}
		note := s.Create(input.Title, input.Body)
		writeJSON(w, http.StatusCreated, note)

	default:
		http.Error(w, `{"error":"метод не поддерживается"}`, http.StatusMethodNotAllowed)
	}
}

func (s *Store) handleNoteByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"метод не поддерживается"}`, http.StatusMethodNotAllowed)
		return
	}

	var id int
	if _, err := fmt.Sscanf(r.URL.Path, "/api/notes/%d", &id); err != nil {
		http.Error(w, `{"error":"некорректный id"}`, http.StatusBadRequest)
		return
	}

	if !s.Delete(id) {
		http.Error(w, `{"error":"заметка не найдена"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

//go:embed index.html
var indexHTML string

// indexHandler отдаёт простой HTML-интерфейс
func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

func main() {
	store := NewStore()

	// Засеваем пару заметок для наглядности
	store.Create("Привет, Go!", "Это первая заметка, созданная при старте сервера.")
	store.Create("Как дела?", "Данные хранятся в памяти — при перезапуске они исчезнут.")

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/api/notes", store.handleNotes)
	mux.HandleFunc("/api/notes/", store.handleNoteByID)

	addr := ":8080"
	log.Printf("Сервер запущен на http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
