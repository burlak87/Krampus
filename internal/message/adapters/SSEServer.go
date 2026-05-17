package adapters

import (
	"context"
	"log"
	"net/http"
	"strconv"

	identityService "krampus/internal/identity/service"
	"krampus/pkg/ctxmeta"
	"krampus/pkg/types"
)

type SSEServer struct {
	authService *identityService.WSAuthService
	manager     *ConnectionManager
	replayRepo  ReplayRepository
}

func NewSSEServer(
	authService *identityService.WSAuthService,
	manager *ConnectionManager,
	replayRepo ReplayRepository,
) *SSEServer {

	return &SSEServer{
		authService: authService,
		manager:     manager,
		replayRepo:  replayRepo,
	}
}

func (s *SSEServer) HandleSSE(
	w http.ResponseWriter,
	r *http.Request,
) {

	roomID := types.RoomID(
		r.URL.Query().Get("room_id"),
	)

	if roomID == "" {

		http.Error(
			w,
			"room_id required",
			http.StatusBadRequest,
		)

		return
	}

	authCtx, err := s.authService.Authenticate(
		r.Context(),
		r,
	)

	if err != nil {

		http.Error(
			w,
			"unauthorized",
			http.StatusUnauthorized,
		)

		return
	}

	meta := ctxmeta.Extract(
		r.Context(),
	)

	meta.UserID = authCtx.UserID.String()

	ctx := ctxmeta.WithMetadata(
		context.Background(),
		meta,
	)

	clientCtx, cancel := context.WithCancel(
		ctx,
	)

	client := NewSSEClient(
		clientCtx,
		cancel,
		w,
		authCtx.UserID,
		roomID,
	)

	if client == nil {

		http.Error(
			w,
			"sse unsupported",
			http.StatusInternalServerError,
		)

		return
	}

	if err := s.manager.Register(client); err != nil {

		http.Error(
			w,
			"registration failed",
			http.StatusInternalServerError,
		)

		return
	}

	defer s.manager.Unregister(client)

	client.Start()

	lastEventID := r.Header.Get(
		"Last-Event-ID",
	)

	if lastEventID == "" {

		lastEventID = r.URL.Query().
			Get("last_event_id")
	}

	if lastEventID != "" {

		lastSequence, err := strconv.ParseInt(
			lastEventID,
			10,
			64,
		)

		if err == nil {

			messages, err := s.replayRepo.
				GetMessagesAfterSequence(
					clientCtx,
					roomID,
					lastSequence,
					100,
				)

			if err == nil {

				for _, replayMsg := range messages {

					event, err := BuildEvent(
						replayMsg.Timestamp,
						replayMsg,
					)

					if err != nil {
						continue
					}

					err = client.SafeSendEvent(
						event,
					)

					if err != nil {
						break
					}
				}
			}
		}
	}

	log.Printf(
		"sse connected trace_id=%s user_id=%s room_id=%s",
		meta.TraceID,
		authCtx.UserID,
		roomID,
	)

	<-clientCtx.Done()
}
