package users

import "context"

const lowScoreThreshold = 0.6

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Overview(ctx context.Context, tenantID, startMs, endMs int64) (UsersOverviewResponse, error) {
	ov, err := s.repo.Overview(ctx, tenantID, startMs, endMs)
	if err != nil {
		return UsersOverviewResponse{}, err
	}
	lowScore, err := s.repo.LowScoreUserCount(ctx, tenantID, startMs, endMs, lowScoreThreshold)
	if err != nil {
		return UsersOverviewResponse{}, err
	}
	resp := UsersOverviewResponse{ActiveUsers: ov.ActiveUsers, LowScoreUsers: lowScore}
	if ov.ActiveUsers > 0 {
		resp.AvgCostPerUser = ov.Cost / float64(ov.ActiveUsers)
		resp.AvgTracesPerUser = float64(ov.Traces) / float64(ov.ActiveUsers)
	}
	return resp, nil
}

func (s *Service) Query(ctx context.Context, tenantID int64, req UsersQueryRequest) (UsersQueryResponse, error) {
	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.repo.TopUsers(ctx, tenantID, req.StartTime, req.EndTime, limit)
	if err != nil {
		return UsersQueryResponse{}, err
	}
	if len(rows) == 0 {
		return UsersQueryResponse{Users: []User{}}, nil
	}
	userIDs := make([]string, len(rows))
	for i, r := range rows {
		userIDs[i] = r.UserID
	}
	scores, err := s.repo.MeanScoreByUser(ctx, tenantID, req.StartTime, req.EndTime, userIDs)
	if err != nil {
		return UsersQueryResponse{}, err
	}
	meanByUser := make(map[string]float64, len(scores))
	for _, sc := range scores {
		meanByUser[sc.UserID] = sc.Mean
	}
	users := make([]User, len(rows))
	for i, r := range rows {
		users[i] = User{
			UserID:     r.UserID,
			TopService: r.TopService,
			Traces:     r.Traces,
			Tokens:     r.Tokens,
			Cost:       r.Cost,
			AvgScore:   meanByUser[r.UserID],
			LastSeenMs: r.LastSeen.UnixMilli(),
		}
	}
	return UsersQueryResponse{Users: users}, nil
}
