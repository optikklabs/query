package sessions

import (
	"context"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/filterutil"

	"golang.org/x/sync/errgroup"
)

const (
	defaultQueryLimit = 50
	maxQueryLimit     = 200
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Overview(ctx context.Context, tenantID, startMs, endMs int64) (SessionsOverviewResponse, error) {
	ov, err := s.repo.Overview(ctx, tenantID, startMs, endMs)
	if err != nil {
		return SessionsOverviewResponse{}, err
	}
	resp := SessionsOverviewResponse{Sessions: ov.Sessions}
	if ov.Sessions > 0 {
		resp.AvgTurns = new(float64(ov.Turns) / float64(ov.Sessions))
		resp.AvgDurationMs = new(ov.DurationMs)
		resp.AvgCost = new(ov.Cost / float64(ov.Sessions))
	}
	return resp, nil
}

func (s *Service) Query(ctx context.Context, tenantID int64, req SessionsQueryRequest) (SessionsQueryResponse, error) {
	if err := filterutil.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return SessionsQueryResponse{}, err
	}
	limit, err := filterutil.Limit(req.Limit, defaultQueryLimit, maxQueryLimit)
	if err != nil {
		return SessionsQueryResponse{}, err
	}
	rows, err := s.repo.TopSessions(ctx, tenantID, req.StartTime, req.EndTime, limit)
	if err != nil {
		return SessionsQueryResponse{}, err
	}
	if len(rows) == 0 {
		return SessionsQueryResponse{Sessions: []Session{}}, nil
	}
	sessionIDs := make([]string, len(rows))
	for i, r := range rows {
		sessionIDs[i] = r.SessionID
	}
	meanBySession, err := s.repo.MeanScoreBySession(ctx, tenantID, req.StartTime, req.EndTime, sessionIDs)
	if err != nil {
		return SessionsQueryResponse{}, err
	}
	out := make([]Session, len(rows))
	for i, r := range rows {
		out[i] = Session{
			SessionID:  r.SessionID,
			Service:    r.Service,
			UserID:     r.UserID,
			Preview:    r.Preview,
			Turns:      r.Turns,
			DurationMs: r.DurationMs,
			Cost:       r.Cost,
			AvgScore:   meanBySession[r.SessionID],
			LastMs:     r.LastTs.UnixMilli(),
		}
	}
	return SessionsQueryResponse{Sessions: out}, nil
}

func (s *Service) Detail(ctx context.Context, tenantID int64, sessionID string, startMs, endMs int64) (SessionDetailResponse, error) {
	var (
		rows     []turnRow
		identity identityRow
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		rows, err = s.repo.Detail(gctx, tenantID, sessionID, startMs, endMs)
		return err
	})
	g.Go(func() error {
		var err error
		identity, err = s.repo.Identity(gctx, tenantID, sessionID, startMs, endMs)
		return err
	})
	if err := g.Wait(); err != nil {
		return SessionDetailResponse{}, err
	}
	if len(rows) == 0 {
		return SessionDetailResponse{}, errorcode.NotFoundError{Msg: "Session not found"}
	}
	resp := SessionDetailResponse{
		SessionID: sessionID,
		Service:   identity.Service,
		UserID:    identity.UserID,
		Turns:     make([]Turn, len(rows)),
	}
	for i, r := range rows {
		resp.Turns[i] = Turn{
			TraceID:    r.TraceID,
			StartMs:    r.Start.UnixMilli(),
			DurationMs: r.DurationMs,
			Model:      r.Model,
			UserText:   r.UserText,
			OutputText: r.OutputText,
			Cost:       r.Cost,
		}
	}
	return resp, nil
}
