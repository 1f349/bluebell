package api

import (
	"context"
	"encoding/json"
	"github.com/1f349/bluebell/database"
	"github.com/1f349/bluebell/upload"
	"github.com/1f349/mjwt"
	"github.com/1f349/mjwt/auth"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/publicsuffix"
	"net/http"
)

type apiDB interface {
	SetDomainBranchEnabled(ctx context.Context, arg database.SetDomainBranchEnabledParams) error
}

func New(upload *upload.Handler, keyStore *mjwt.KeyStore, db apiDB) *httprouter.Router {
	router := httprouter.New()
	router.POST("/u/:site/:branch", upload.Handle)
	router.PUT("/sites/:host/:branch/enable", checkAuth(keyStore, func(rw http.ResponseWriter, req *http.Request, params httprouter.Params, b AuthClaims) {
		setEnabled(rw, req, params, b, db, true)
	}))
	router.DELETE("/sites/:host/:branch/enable", checkAuth(keyStore, func(rw http.ResponseWriter, req *http.Request, params httprouter.Params, b AuthClaims) {
		setEnabled(rw, req, params, b, db, false)
	}))
	return router
}

func setEnabled(rw http.ResponseWriter, req *http.Request, params httprouter.Params, b AuthClaims, db apiDB, enable bool) {
	host := params.ByName("host")
	branch := params.ByName("branch")

	if !validateDomainOwnershipClaims(host, b.Claims.Perms) {
		http.Error(rw, "Forbidden", http.StatusForbidden)
	}

	err := db.SetDomainBranchEnabled(req.Context(), database.SetDomainBranchEnabledParams{
		Domain: host,
		Branch: branch,
		Enable: enable,
	})
	if err != nil {
		http.Error(rw, "Failed to update branch state", http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusAccepted)
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
