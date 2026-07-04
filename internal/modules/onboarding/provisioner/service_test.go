package provisioner

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type fakeRepo struct {
	pending  []PendingTeam
	statuses map[int64]string
}

func (f *fakeRepo) ListPending(context.Context) ([]PendingTeam, error) { return f.pending, nil }
func (f *fakeRepo) SetStatus(_ context.Context, teamID int64, status string) error {
	f.statuses[teamID] = status
	return nil
}

type fakeApplier struct {
	applied   int
	applyErr  error
	available bool
}

func (f *fakeApplier) Apply(_ context.Context, objs []*unstructured.Unstructured) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applied += len(objs)
	return nil
}

func (f *fakeApplier) CollectorAvailable(context.Context, string, string) (bool, error) {
	return f.available, nil
}

func TestTick(t *testing.T) {
	valid := PendingTeam{ID: 3, Slug: "acme-roc", APIKey: "c3448fae"}
	cases := []struct {
		name       string
		team       PendingTeam
		applier    fakeApplier
		wantStatus string
		wantApply  int
	}{
		{name: "ready when collector available", team: valid,
			applier: fakeApplier{available: true}, wantStatus: "ready", wantApply: 7},
		{name: "stays pending until available", team: valid,
			applier: fakeApplier{available: false}, wantStatus: "", wantApply: 7},
		{name: "stays pending on apply failure", team: valid,
			applier: fakeApplier{applyErr: errors.New("boom")}, wantStatus: "", wantApply: 0},
		{name: "error on unrenderable slug", team: PendingTeam{ID: 4, Slug: "Bad.Slug", APIKey: "c3448fae"},
			applier: fakeApplier{available: true}, wantStatus: "error", wantApply: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{pending: []PendingTeam{tc.team}, statuses: map[int64]string{}}
			svc := NewService(repo, &tc.applier)
			if err := svc.Tick(context.Background()); err != nil {
				t.Fatalf("Tick failed: %v", err)
			}
			if got := repo.statuses[tc.team.ID]; got != tc.wantStatus {
				t.Errorf("status = %q, want %q", got, tc.wantStatus)
			}
			if tc.applier.applied != tc.wantApply {
				t.Errorf("applied %d objects, want %d", tc.applier.applied, tc.wantApply)
			}
		})
	}
}
