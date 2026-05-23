package upload

import (
	"context"
	"database/sql"
)

type SessionRepository interface {
	GetChunks(
		ctx context.Context,
		sessionID string,
	) ([]Chunk, error)

	CompleteSession(
		ctx context.Context,
		tx *sql.Tx,
		sessionID string,
	) error
}

type Finalizer struct {
	db *sql.DB

	repo     SessionRepository
	verifier *ResumeVerifier
}

func NewFinalizer(
	db *sql.DB,
	repo SessionRepository,
	verifier *ResumeVerifier,
) *Finalizer {

	return &Finalizer{
		db:       db,
		repo:     repo,
		verifier: verifier,
	}
}

func (f *Finalizer) Finalize(
	ctx context.Context,
	sessionID string,
	expectedChunks int,
	expectedHash string,
) error {

	tx, err := f.db.BeginTx(
		ctx,
		nil,
	)

	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = LockSession(
		ctx,
		tx,
		sessionID,
	)

	if err != nil {
		return err
	}

	chunks, err := f.repo.GetChunks(
		ctx,
		sessionID,
	)

	if err != nil {
		return err
	}

	err = VerifyChunkCount(
		expectedChunks,
		len(chunks),
	)

	if err != nil {
		return err
	}

	for _, chunk := range chunks {

		ok := f.verifier.VerifyChunkExists(
			ctx,
			chunk,
		)

		if !ok {
			return ErrMissingChunks
		}
	}

	var merged []byte

	for _, chunk := range chunks {
		merged = append(
			merged,
			chunk.Data...,
		)
	}

	actualHash := SHA256(
		merged,
	)

	err = VerifyUploadHash(
		actualHash,
		expectedHash,
	)

	if err != nil {
		return err
	}

	err = f.repo.CompleteSession(
		ctx,
		tx,
		sessionID,
	)

	if err != nil {
		return err
	}

	return tx.Commit()
}
