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
// Only the methods the non-transactional handlers call are implemented with real
// behavior; the rest are stubs (register uses a pgx transaction, not this).
type fakeStore struct {
	usersByEmail map[string]store.User
	usersByID    map[int64]store.User
	profiles     map[int64]store.GetDonorProfileRow
	screenings   map[int64][]store.ScreeningResponse
}

var _ store.Querier = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	return &fakeStore{
		usersByEmail: map[string]store.User{},
		usersByID:    map[int64]store.User{},
		profiles:     map[int64]store.GetDonorProfileRow{},
		screenings:   map[int64][]store.ScreeningResponse{},
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

// --- stubs (not reached by the handlers under test) ---

func (f *fakeStore) CreateUser(context.Context, store.CreateUserParams) (store.CreateUserRow, error) {
	return store.CreateUserRow{}, pgx.ErrNoRows
}
func (f *fakeStore) CreateDonorProfile(context.Context, store.CreateDonorProfileParams) (store.CreateDonorProfileRow, error) {
	return store.CreateDonorProfileRow{}, pgx.ErrNoRows
}
func (f *fakeStore) EmailExists(context.Context, string) (bool, error) { return false, nil }
func (f *fakeStore) CreateClinic(context.Context, store.CreateClinicParams) (store.CreateClinicRow, error) {
	return store.CreateClinicRow{}, pgx.ErrNoRows
}
func (f *fakeStore) GetClinic(context.Context, int64) (store.GetClinicRow, error) {
	return store.GetClinicRow{}, pgx.ErrNoRows
}
func (f *fakeStore) AddClinicMember(context.Context, store.AddClinicMemberParams) (store.ClinicMember, error) {
	return store.ClinicMember{}, pgx.ErrNoRows
}
func (f *fakeStore) GetClinicMembershipByUser(context.Context, int64) (store.ClinicMember, error) {
	return store.ClinicMember{}, pgx.ErrNoRows
}
