package http

import (
	"context"
	"time"

	"github.com/blood-donation/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// fakeStore is an in-memory store.Querier for handler tests, so we can exercise
// the HTTP layer (validation, auth, RBAC, response shapes) without a database.
// Only the methods the non-transactional handlers under test call are implemented
// with real behavior. The embedded store.Querier satisfies the rest of the
// interface; any unimplemented method panics if a test unexpectedly calls it,
// which keeps this fake from breaking the build every time a query is added.
type fakeStore struct {
	store.Querier
	usersByEmail map[string]store.User
	usersByID    map[int64]store.User
	profiles     map[int64]store.GetDonorProfileRow
	screenings   map[int64][]store.ScreeningResponse
	memberships  map[int64]store.ClinicMember
	requests     map[int64]store.GetBloodRequestRow
	nextReqID    int64
	matchCount   int // how many matches MatchRequestDonors returns
}

var _ store.Querier = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	return &fakeStore{
		usersByEmail: map[string]store.User{},
		usersByID:    map[int64]store.User{},
		profiles:     map[int64]store.GetDonorProfileRow{},
		screenings:   map[int64][]store.ScreeningResponse{},
		memberships:  map[int64]store.ClinicMember{},
		requests:     map[int64]store.GetBloodRequestRow{},
	}
}

func (f *fakeStore) addUser(u store.User) {
	f.usersByEmail[u.Email] = u
	f.usersByID[u.ID] = u
}

// --- implemented behavior ---

func (f *fakeStore) GetUserByEmail(_ context.Context, email string) (store.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return store.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (f *fakeStore) GetUserByID(_ context.Context, id int64) (store.GetUserByIDRow, error) {
	u, ok := f.usersByID[id]
	if !ok {
		return store.GetUserByIDRow{}, pgx.ErrNoRows
	}
	return store.GetUserByIDRow{
		ID: u.ID, Email: u.Email, Role: u.Role, FullName: u.FullName,
		Phone: u.Phone, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}, nil
}

func (f *fakeStore) GetDonorProfile(_ context.Context, userID int64) (store.GetDonorProfileRow, error) {
	p, ok := f.profiles[userID]
	if !ok {
		return store.GetDonorProfileRow{}, pgx.ErrNoRows
	}
	return p, nil
}

func (f *fakeStore) UpdateDonorProfile(_ context.Context, arg store.UpdateDonorProfileParams) (store.UpdateDonorProfileRow, error) {
	p, ok := f.profiles[arg.UserID]
	if !ok {
		return store.UpdateDonorProfileRow{}, pgx.ErrNoRows
	}
	p.BloodType = arg.BloodType
	p.IsAvailable = arg.IsAvailable
	p.TravelRadiusM = arg.TravelRadiusM
	p.Latitude = arg.Latitude
	p.Longitude = arg.Longitude
	p.LastDonationDate = arg.LastDonationDate
	f.profiles[arg.UserID] = p
	return store.UpdateDonorProfileRow(p), nil
}

func (f *fakeStore) CreateScreeningResponse(_ context.Context, arg store.CreateScreeningResponseParams) (store.ScreeningResponse, error) {
	row := store.ScreeningResponse{
		ID:          int64(len(f.screenings[arg.DonorUserID]) + 1),
		DonorUserID: arg.DonorUserID,
		Answers:     arg.Answers,
		HasFlags:    arg.HasFlags,
		SubmittedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	f.screenings[arg.DonorUserID] = append(f.screenings[arg.DonorUserID], row)
	return row, nil
}

func (f *fakeStore) GetLatestScreeningResponse(_ context.Context, donorUserID int64) (store.ScreeningResponse, error) {
	rows := f.screenings[donorUserID]
	if len(rows) == 0 {
		return store.ScreeningResponse{}, pgx.ErrNoRows
	}
	return rows[len(rows)-1], nil
}

// --- requests, matching & notifications (PR3) ---

func (f *fakeStore) GetClinicMembershipByUser(_ context.Context, userID int64) (store.ClinicMember, error) {
	m, ok := f.memberships[userID]
	if !ok {
		return store.ClinicMember{}, pgx.ErrNoRows
	}
	return m, nil
}

func (f *fakeStore) CreateBloodRequest(_ context.Context, arg store.CreateBloodRequestParams) (store.CreateBloodRequestRow, error) {
	f.nextReqID++
	row := store.GetBloodRequestRow{
		ID:          f.nextReqID,
		ClinicID:    arg.ClinicID,
		CreatedBy:   arg.CreatedBy,
		BloodType:   arg.BloodType,
		UnitsNeeded: arg.UnitsNeeded,
		Urgency:     arg.Urgency,
		Status:      store.RequestStatusPendingApproval,
		RadiusM:     arg.RadiusM,
	}
	f.requests[row.ID] = row
	return store.CreateBloodRequestRow(row), nil
}

func (f *fakeStore) GetBloodRequest(_ context.Context, id int64) (store.GetBloodRequestRow, error) {
	row, ok := f.requests[id]
	if !ok {
		return store.GetBloodRequestRow{}, pgx.ErrNoRows
	}
	return row, nil
}

func (f *fakeStore) ApproveBloodRequest(_ context.Context, arg store.ApproveBloodRequestParams) (store.ApproveBloodRequestRow, error) {
	row, ok := f.requests[arg.ID]
	if !ok || row.Status != store.RequestStatusPendingApproval {
		// Mirrors the SQL status guard: only a pending request transitions.
		return store.ApproveBloodRequestRow{}, pgx.ErrNoRows
	}
	row.Status = store.RequestStatusApproved
	row.ApprovedBy = arg.ApprovedBy
	f.requests[arg.ID] = row
	return store.ApproveBloodRequestRow(row), nil
}

func (f *fakeStore) MatchRequestDonors(_ context.Context, arg store.MatchRequestDonorsParams) ([]store.DonorMatch, error) {
	out := make([]store.DonorMatch, f.matchCount)
	for i := range out {
		out[i] = store.DonorMatch{ID: int64(i + 1), RequestID: arg.RequestID, DonorUserID: int64(i + 1)}
	}
	return out, nil
}

func (f *fakeStore) CreateActivityLog(_ context.Context, _ store.CreateActivityLogParams) (store.ActivityLog, error) {
	return store.ActivityLog{}, nil
}

func (f *fakeStore) ListPushTokensForUser(_ context.Context, _ int64) ([]store.PushToken, error) {
	return nil, nil
}
