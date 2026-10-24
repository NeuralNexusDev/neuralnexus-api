package twitch

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

var ErrEventSubSubscriptionNotFound = errors.New("eventsub subscription not found")

// store struct for storing Twitch data
type store struct {
	db *pgxpool.Pool
}

// NewStore creates a new Twitch store
func NewStore(db *pgxpool.Pool) Store {
	return &store{db: db}
}

// EventSubEntry struct for storing EventSub subscription data
type EventSubEntry struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Status    string    `json:"status" db:"status"`
	Type      string    `json:"type" db:"type"`
	Version   int       `json:"version" db:"version"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	RevokedAt time.Time `json:"revoked_at" db:"revoked_at"`
	Cost      int       `json:"cost" db:"cost"`
}

// Store struct for storing Twitch data
type Store interface {
	GetEventSubSubscription(ctx context.Context, id string) (*EventSubEntry, error)
	GetEventSubSubscriptions(ctx context.Context, userID string) ([]*EventSubEntry, error)
	CreateEventSubSubscription(ctx context.Context, entry *EventSubEntry) error
	UpdateEventSubSubscription(ctx context.Context, entry *EventSubEntry) error
}

// GetEventSubSubscription gets an EventSub subscription by ID
func (s *store) GetEventSubSubscription(ctx context.Context, id string) (*EventSubEntry, error) {
	rows, err := s.db.Query(ctx, "SELECT * FROM eventsub_subscriptions WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entry, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[EventSubEntry])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventSubSubscriptionNotFound
		}
		return nil, err
	}
	return entry, nil
}

// GetEventSubSubscriptions gets all EventSub subscriptions for a user
func (s *store) GetEventSubSubscriptions(ctx context.Context, userID string) ([]*EventSubEntry, error) {
	rows, err := s.db.Query(ctx, "SELECT * FROM eventsub_subscriptions WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[EventSubEntry])
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// CreateEventSubSubscription creates a new EventSub subscription
func (s *store) CreateEventSubSubscription(ctx context.Context, entry *EventSubEntry) error {
	_, err := s.db.Exec(ctx, "INSERT INTO eventsub_subscriptions (id, user_id, status, type, version, created_at, revoked_at, cost) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		entry.ID, entry.UserID, entry.Status, entry.Type, entry.Version, entry.CreatedAt, entry.RevokedAt, entry.Cost)
	if err != nil {
		return err
	}
	return nil
}

// UpdateEventSubSubscription updates an EventSub subscription
func (s *store) UpdateEventSubSubscription(ctx context.Context, entry *EventSubEntry) error {
	_, err := s.db.Exec(ctx, "UPDATE eventsub_subscriptions SET status = $1, type = $2, version = $3, created_at = $4, revoked_at = $5, cost = $6 WHERE id = $7",
		entry.Status, entry.Type, entry.Version, entry.CreatedAt, entry.RevokedAt, entry.Cost, entry.ID)
	if err != nil {
		return err
	}
	return nil
}

// EventSubService interface for handling EventSub subscriptions
type EventSubService interface {
	GetEventSubSubscription(ctx context.Context, id string) (*EventSubEntry, error)
	GetEventSubSubscriptions(ctx context.Context, userID string) ([]*EventSubEntry, error)
	CreateEventSubSubscription(ctx context.Context, entry *EventSubEntry) error
	UpdateEventSubSubscriptionStatus(ctx context.Context, id string, status string) error
	RevokeEventSubSubscription(ctx context.Context, id string, status string) error
}

// service struct for handling EventSub subscriptions
type service struct {
	store Store
}

// NewService creates a new EventSub service
func NewService(store Store) EventSubService {
	return &service{store: store}
}

// GetEventSubSubscription gets an EventSub subscription by ID
func (s *service) GetEventSubSubscription(ctx context.Context, id string) (*EventSubEntry, error) {
	return s.store.GetEventSubSubscription(ctx, id)
}

// GetEventSubSubscriptions gets all EventSub subscriptions for a user
func (s *service) GetEventSubSubscriptions(ctx context.Context, userID string) ([]*EventSubEntry, error) {
	return s.store.GetEventSubSubscriptions(ctx, userID)
}

// CreateEventSubSubscription creates a new EventSub subscription
func (s *service) CreateEventSubSubscription(ctx context.Context, entry *EventSubEntry) error {
	return s.store.CreateEventSubSubscription(ctx, entry)
}

// UpdateEventSubSubscriptionStatus updates an EventSub subscription's status
func (s *service) UpdateEventSubSubscriptionStatus(ctx context.Context, id string, status string) error {
	entry, err := s.store.GetEventSubSubscription(ctx, id)
	if err != nil {
		return err
	}
	entry.Status = status
	return s.store.UpdateEventSubSubscription(context.WithoutCancel(ctx), entry)
}

// RevokeEventSubSubscription revokes an EventSub subscription
func (s *service) RevokeEventSubSubscription(ctx context.Context, id string, status string) error {
	entry, err := s.store.GetEventSubSubscription(ctx, id)
	if err != nil {
		return err
	}
	entry.Status = status
	entry.RevokedAt = time.Now()
	return s.store.UpdateEventSubSubscription(context.WithoutCancel(ctx), entry)
}
