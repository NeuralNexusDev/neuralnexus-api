package events

import (
	"context"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

// CREATE DATABASE events;

// CREATE TRIGGER update_event_log_modtime
// BEFORE UPDATE ON event_log
// FOR EACH ROW
// EXECUTE PROCEDURE update_modified_column();

// CREATE TABLE event_log (
//    id BIG INT PRIMARY KEY NOT NULL,
//    user_id INT NOT NULL,
//    platform TEXT NOT NULL,
//    type TEXT NOT NULL,
//    status INT DEFAULT 0,
//    payload JSONB,
//    created_at TIMESTAMPTZ DEFAULT now(),
//    updated_at TIMESTAMPTZ DEFAULT now()
//   CONSTRAINT events_unique UNIQUE (id)
// );

type EventType string

type EventStatus int

const (
	Pending   EventStatus = 0
	Fulfilled EventStatus = 1
	Rejected  EventStatus = 2
	Error     EventStatus = 3
)

// Event represents a generic event in the system
type Event struct {
	ID        string          `json:"id" db:"id"`
	UserID    string          `json:"user_id" db:"user_id"`
	Platform  auth.Platform   `json:"platform" db:"platform"`
	Type      EventType       `json:"type" db:"type"`
	Status    EventStatus     `json:"status" db:"status"`
	Payload   json.RawMessage `json:"payload" db:"payload"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// NewEvent creates a new event
func NewEvent(userId string, platform auth.Platform, eventType EventType, payload json.RawMessage) (*Event, error) {
	id, err := database.GenSnowflake()
	if err != nil {
		return nil, err
	}
	return &Event{
		ID:       id,
		UserID:   userId,
		Platform: platform,
		Type:     eventType,
		Status:   Pending,
		Payload:  payload,
	}, nil
}

// EventStore interface for storing events
type EventStore interface {
	GetEvent(id string) (*Event, error)
	GetEventsByPlatform(platform auth.Platform) ([]*Event, error)
	CreateEvent(event *Event) error
	UpdateEvent(event *Event) error
}

// store implements the EventStore interface
type store struct {
	db *pgxpool.Pool
}

// NewEventStore creates a new EventStore
func NewEventStore(db *pgxpool.Pool) EventStore {
	return &store{db: db}
}

// GetEvent retrieves an event by its ID
func (s *store) GetEvent(id string) (*Event, error) {
	var event Event
	err := s.db.QueryRow(context.Background(), "SELECT * FROM event_log WHERE id = $1", id).Scan(
		&event.ID, &event.Platform, &event.Type, &event.Payload, &event.Status, &event.CreatedAt, &event.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// GetEventsByPlatform retrieves all events for a specific platform
func (s *store) GetEventsByPlatform(platform auth.Platform) ([]*Event, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM event_log WHERE platform = $1", platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	events, err = pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[Event])
	return events, nil
}

// CreateEvent inserts a new event into the database
func (s *store) CreateEvent(event *Event) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO event_log (id, platform, type, payload, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		event.ID, event.Platform, event.Type, event.Payload, event.Status, event.CreatedAt, event.UpdatedAt)
	return err
}

// UpdateEvent updates an existing event in the database
func (s *store) UpdateEvent(event *Event) error {
	_, err := s.db.Exec(context.Background(), "UPDATE event_log SET platform = $1, type = $2, payload = $3, status = $4, updated_at = $5 WHERE id = $6",
		event.Platform, event.Type, event.Payload, event.Status, time.Now(), event.ID)
	return err
}

// EventService interface for handling events
type EventService interface {
	GetEvent(id string) (*Event, error)
	GetEventsByPlatform(platform auth.Platform) ([]*Event, error)
	CreateEvent(event *Event) error
	UpdateEventStatus(id string, status EventStatus) error
}

// service implements the EventService interface
type service struct {
	store EventStore
}

// NewEventService creates a new EventService
func NewEventService(store EventStore) EventService {
	return &service{store: store}
}

// GetEvent retrieves an event by its ID
func (s *service) GetEvent(id string) (*Event, error) {
	return s.store.GetEvent(id)
}

// GetEventsByPlatform retrieves all events for a specific platform
func (s *service) GetEventsByPlatform(platform auth.Platform) ([]*Event, error) {
	return s.store.GetEventsByPlatform(platform)
}

// CreateEvent creates a new event
func (s *service) CreateEvent(event *Event) error {
	return s.store.CreateEvent(event)
}

// UpdateEventStatus updates the status of an event
func (s *service) UpdateEventStatus(id string, status EventStatus) error {
	event, err := s.store.GetEvent(id)
	if err != nil {
		return err
	}
	event.Status = status
	return s.store.UpdateEvent(event)
}
