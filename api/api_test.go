package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/1f349/bluebell/database"
	"github.com/1f349/mjwt"
	"github.com/1f349/mjwt/auth"
	"github.com/golang-jwt/jwt/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestApi(t *testing.T) {
	keystore := mjwt.NewKeyStoreWithDir(afero.NewMemMapFs())
	issuer, err := mjwt.NewIssuerWithKeyStore("test", "1", jwt.SigningMethodRS512, keystore)
	if err != nil {
		panic(err)
	}

	psValid := auth.NewPermStorage()
	psValid.Set("domain:owns=example.com")

	psInvalid := auth.NewPermStorage()
	psInvalid.Set("domain:owns=example.org")

	validToken, err := issuer.GenerateJwt("1", "2", jwt.ClaimStrings{"3"}, time.Hour, auth.AccessTokenClaims{Perms: psValid})
	if err != nil {
		panic(err)
	}
	invalidToken, err := issuer.GenerateJwt("1", "2", jwt.ClaimStrings{"3"}, time.Hour, auth.AccessTokenClaims{Perms: psInvalid})
	if err != nil {
		panic(err)
	}

	mux := New(&fakeUpload{}, keystore, &fakeDB{})

	t.Run("GET /", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "Bluebell API Endpoint\n", rec.Body.String())
	})

	t.Run("POST /u/example.com/main", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/u/example.com/main", nil))
		assert.Equal(t, 788, rec.Code)
		assert.Equal(t, "fakeUpload.Handle called\n", rec.Body.String())
	})
	t.Run("POST /u", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/u", nil))
		assert.Equal(t, 788, rec.Code)
		assert.Equal(t, "fakeUpload.Handle called\n", rec.Body.String())
	})

	t.Run("GET /sites/example.com", func(t *testing.T) {
		t.Run("Invalid", func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/sites/example.com", nil)
			req.Header.Set("Authorization", "Bearer "+invalidToken)
			mux.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Equal(t, "Forbidden\n", rec.Body.String())
		})
		t.Run("Valid", func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/sites/example.com", nil)
			req.Header.Set("Authorization", "Bearer "+validToken)
			mux.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "[{\"domain\":\"example.com\",\"branches\":[{\"domain\":\"example.com\",\"branch\":\"test\",\"last_update\":\"2000-02-08T01:02:03Z\",\"enable\":true,\"archive\":\"\"}]}]\n", rec.Body.String())
		})
	})
}

type fakeUpload struct{}

func (f *fakeUpload) Handle(rw http.ResponseWriter, req *http.Request, params httprouter.Params) {
	rw.WriteHeader(788)
	rw.Write([]byte("fakeUpload.Handle called\n"))
}

type fakeDB struct{}

func (f *fakeDB) GetBranchesByHost(ctx context.Context, arg database.GetBranchesByHostParams) ([]database.Branch, error) {
	if arg.Domain == "example.com" && arg.DomainWildcard == "%.example.com" {
		return []database.Branch{
			{Domain: "example.com", Branch: "test", LastUpdate: time.Date(2000, time.February, 8, 1, 2, 3, 0, time.UTC), Enable: true},
		}, nil
	}
	return []database.Branch{}, nil
}

func (f *fakeDB) AddSite(ctx context.Context, arg database.AddSiteParams) error {
	//TODO implement me
	panic("implement me")
}

func (f *fakeDB) UpdateSiteToken(ctx context.Context, arg database.UpdateSiteTokenParams) error {
	//TODO implement me
	panic("implement me")
}

func (f *fakeDB) SetBranchEnabled(ctx context.Context, arg database.SetBranchEnabledParams) error {
	//TODO implement me
	panic("implement me")
}
