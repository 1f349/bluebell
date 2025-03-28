package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/1f349/bluebell/database"
	"github.com/1f349/bluebell/upload"
	"github.com/1f349/bluebell/validation"
	"github.com/1f349/mjwt"
	"github.com/1f349/mjwt/auth"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/publicsuffix"
	"net/http"
)

type apiDB interface {
	GetBranchesByHost(ctx context.Context, arg database.GetBranchesByHostParams) ([]database.Branch, error)
	AddSite(ctx context.Context, arg database.AddSiteParams) error
	UpdateSiteToken(ctx context.Context, arg database.UpdateSiteTokenParams) error
	SetBranchEnabled(ctx context.Context, arg database.SetBranchEnabledParams) error
}

func New(upload *upload.Handler, keyStore *mjwt.KeyStore, db apiDB) *httprouter.Router {
	router := httprouter.New()

	router.GET("/", func(rw http.ResponseWriter, req *http.Request, params httprouter.Params) {
		http.Error(rw, "Bluebell API Endpoint", http.StatusOK)
	})

	// Site upload endpoint
	router.POST("/u/:site/:branch", upload.Handle)
	router.POST("/u", func(rw http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		q := req.URL.Query()
		upload.Handle(rw, req, httprouter.Params{
			{"site", q.Get("site")},
			{"branch", q.Get("branch")},
		})
	})

	// Site lookup endpoint
	router.GET("/api/v1/sites/:host", checkAuth(keyStore, func(rw http.ResponseWriter, req *http.Request, params httprouter.Params, b AuthClaims) {
		host := params.ByName("host")

		if !validation.IsValidHost(host) {
			http.Error(rw, "Invalid host", http.StatusBadRequest)
			return
		}

		if !validateDomainOwnershipClaims(host, b.Claims.Perms) {
			http.Error(rw, "Forbidden", http.StatusForbidden)
			return
		}

		branches, err := db.GetBranchesByHost(req.Context(), database.GetBranchesByHostParams{
			Domain:         host,
			DomainWildcard: "%." + host,
		})
		if err != nil {
			http.Error(rw, "Failed to fetch sites", http.StatusInternalServerError)
			return
		}

		domains := make(map[string][]database.Branch)
		for _, i := range branches {
			domains[i.Domain] = append(domains[i.Domain], i)
		}

		type domainObject struct {
			Domain   string            `json:"domain"`
			Branches []database.Branch `json:"branches"`
		}

		domainObjs := make([]domainObject, 0, len(domains))
		for k, v := range domains {
			domainObjs = append(domainObjs, domainObject{
				Domain:   k,
				Branches: v,
			})
		}

		rw.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(rw).Encode(domainObjs)
	}))

	// Site creation endpoint
	router.PUT("/api/v1/sites/:host", checkAuth(keyStore, func(rw http.ResponseWriter, req *http.Request, params httprouter.Params, b AuthClaims) {
		host := params.ByName("host")

		if !validation.IsValidSite(host) {
			http.Error(rw, "Invalid site", http.StatusBadRequest)
			return
		}

		if !validateDomainOwnershipClaims(host, b.Claims.Perms) {
			http.Error(rw, "Forbidden", http.StatusForbidden)
			return
		}

		token, err := generateToken()
		if err != nil {
			http.Error(rw, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		err = db.AddSite(req.Context(), database.AddSiteParams{
			Domain: host,
			Token:  token,
		})
		if err != nil {
			http.Error(rw, "Failed to register site", http.StatusInternalServerError)
			return
		}

		rw.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(rw).Encode(struct {
			Token string `json:"token"`
		}{Token: token})
	}))

	// Reset site token endpoint
	router.POST("/api/v1/sites/:host/reset-token", checkAuth(keyStore, func(rw http.ResponseWriter, req *http.Request, params httprouter.Params, b AuthClaims) {
		host := params.ByName("host")

		if !validation.IsValidSite(host) {
			http.Error(rw, "Invalid site", http.StatusBadRequest)
			return
		}

		if !validateDomainOwnershipClaims(host, b.Claims.Perms) {
			http.Error(rw, "Forbidden", http.StatusForbidden)
			return
		}

		token, err := generateToken()
		if err != nil {
			http.Error(rw, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		err = db.UpdateSiteToken(req.Context(), database.UpdateSiteTokenParams{
			Domain: host,
			Token:  token,
		})
		if err != nil {
			http.Error(rw, "Failed to register site", http.StatusInternalServerError)
			return
		}

		rw.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(rw).Encode(struct {
			Token string `json:"token"`
		}{Token: token})
	}))

	// Enable/disable site branch
	router.PUT("/api/v1/sites/:host/:branch/enable", checkAuth(keyStore, func(rw http.ResponseWriter, req *http.Request, params httprouter.Params, b AuthClaims) {
		setEnabled(rw, req, params, b, db, true)
	}))
	router.DELETE("/api/v1/sites/:host/:branch/enable", checkAuth(keyStore, func(rw http.ResponseWriter, req *http.Request, params httprouter.Params, b AuthClaims) {
		setEnabled(rw, req, params, b, db, false)
	}))

	return router
}

func setEnabled(rw http.ResponseWriter, req *http.Request, params httprouter.Params, b AuthClaims, db apiDB, enable bool) {
	host := params.ByName("host")
	branch := params.ByName("branch")

	if !validation.IsValidSite(host) {
		http.Error(rw, "Invalid site", http.StatusBadRequest)
		return
	}

	if !validation.IsValidBranch(branch) {
		http.Error(rw, "Invalid branch", http.StatusBadRequest)
		return
	}

	if !validateDomainOwnershipClaims(host, b.Claims.Perms) {
		http.Error(rw, "Forbidden", http.StatusForbidden)
		return
	}

	err := db.SetBranchEnabled(req.Context(), database.SetBranchEnabledParams{
		Domain: host,
		Branch: branch,
		Enable: enable,
	})
	if err != nil {
		http.Error(rw, "Failed to update branch state", http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
}

func generateToken() (string, error) {
	var b [32]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// apiError outputs a generic JSON error message
func apiError(rw http.ResponseWriter, code int, m string) {
	rw.WriteHeader(code)
	_ = json.NewEncoder(rw).Encode(map[string]string{
		"error": m,
	})
}

// validateDomainOwnershipClaims validates if the claims contain the
// `domain:owns=<fqdn>` field with the matching top level domain
func validateDomainOwnershipClaims(a string, perms *auth.PermStorage) bool {
	if fqdn, err := publicsuffix.EffectiveTLDPlusOne(a); err == nil {
		if perms.Has("domain:owns=" + fqdn) {
			return true
		}
	}
	return false
}
