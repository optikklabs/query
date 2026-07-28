package sessions

import "context"

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
	resp := SessionsOverviewResponse{Sessions: ov.Sessions, AvgDurationMs: ov.DurationMs}
	if ov.Sessions > 0 {
		resp.AvgTurns = float64(ov.Turns) / float64(ov.Sessions)
		resp.AvgCost = ov.Cost / float64(ov.Sessions)
	}
	return resp, nil
}

func (s *Service) Query(ctx context.Context, tenantID int64, req SessionsQueryRequest) (SessionsQueryResponse, error) {
	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
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
	scores, err := s.repo.MeanScoreBySession(ctx, tenantID, req.StartTime, req.EndTime, sessionIDs)
	if err != nil {
		return SessionsQueryResponse{}, err
	}
	meanBySession := make(map[string]float64, len(scores))
	for _, sc := range scores {
		meanBySession[sc.SessionID] = sc.Mean
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
	rows, err := s.repo.Detail(ctx, tenantID, sessionID, startMs, endMs)
	if err != nil {
		return SessionDetailResponse{}, err
	}
	resp := SessionDetailResponse{SessionID: sessionID, Turns: make([]Turn, len(rows))}
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
