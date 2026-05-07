package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"krampus/internal/chat/domain"
	database "krampus/internal/sqlc"
)

type RoomPGStorage struct {
	queries *database.Queries
}

func NewRoomPGStorage(queries *database.Queries) *RoomPGStorage {
	return &RoomPGStorage{queries: queries}
}

func (s *RoomPGStorage) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	row, err := s.queries.GetRoomByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	var members []string
	if err := json.Unmarshal(row.Members, &members); err != nil {
		return nil, fmt.Errorf("failed to unmarshal members: %w", err)
	}

	var settings domain.RoomSettings
	if err := json.Unmarshal(row.Settings, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &domain.Room{
		ID:        row.ID,
		Type:      domain.RoomType(row.Type),
		OwnerID:   row.OwnerID,
		Name:      row.Name,
		Members:   members,
		Settings:  settings,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *RoomPGStorage) SaveRoom(ctx context.Context, room *domain.Room) error {
	membersJSON, err := json.Marshal(room.Members)
	if err != nil {
		return err
	}
	settingsJSON, err := json.Marshal(room.Settings)
	if err != nil {
		return err
	}

	return s.queries.UpsertRoom(ctx, database.UpsertRoomParams{
		ID:        room.ID,
		Type:      string(room.Type),
		OwnerID:   room.OwnerID,
		Name:      room.Name,
		Members:   membersJSON,
		Settings:  settingsJSON,
		CreatedAt: room.CreatedAt,
		UpdatedAt: room.UpdatedAt,
	})
}

func (s *RoomPGStorage) UpdateRoom(ctx context.Context, room *domain.Room) error {
	membersJSON, err := json.Marshal(room.Members)
	if err != nil {
		return err
	}
	settingsJSON, err := json.Marshal(room.Settings)
	if err != nil {
		return err
	}

	return s.queries.UpdateRoom(ctx, database.UpdateRoomParams{
		ID:        room.ID,
		Name:      room.Name,
		Members:   membersJSON,
		Settings:  settingsJSON,
		UpdatedAt: time.Now().UnixNano(),
	})
}

func (s *RoomPGStorage) DeleteRoom(ctx context.Context, id string) error {
	return s.queries.DeleteRoom(ctx, id)
}

func (s *RoomPGStorage) ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	arg := fmt.Sprintf("[\"%s\"]", userID)
	rows, err := s.queries.ListUserRooms(ctx, []byte(arg))
	if err != nil {
		return nil, err
	}

	var rooms []*domain.Room
	for _, row := range rows {
		var members []string
		_ = json.Unmarshal(row.Members, &members)
		var settings domain.RoomSettings
		_ = json.Unmarshal(row.Settings, &settings)

		rooms = append(rooms, &domain.Room{
			ID:        row.ID,
			Type:      domain.RoomType(row.Type),
			OwnerID:   row.OwnerID,
			Name:      row.Name,
			Members:   members,
			Settings:  settings,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return rooms, nil
}
