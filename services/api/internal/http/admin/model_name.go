package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
)

var publicModelNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)

func (s *Server) modelName(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		s.err(w, 400, "invalid_parameter")
		return
	}
	next, expected, ok := decodeModelName(r)
	if !ok {
		s.err(w, 400, "invalid_parameter")
		return
	}
	if !s.fence.Alive() {
		s.err(w, 503, "authority_unavailable")
		return
	}
	version, e := s.store.SetPublicModelID(r.Context(), s.fence.Holder(), s.fence.Epoch(), expected, next)
	if e != nil {
		var pg *pgconn.PgError
		if errors.As(e, &pg) && pg.Code == "P0002" {
			s.err(w, 409, "stale_public_model_id")
			return
		}
		s.err(w, 503, "database_unavailable")
		return
	}
	writeJSON(w, 200, map[string]any{"public_model_id": next, "snapshot_version": version})
}

func decodeModelName(r *http.Request) (string, string, bool) {
	media, params, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
		return "", "", false
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, 1025))
	if e != nil || len(raw) > 1024 || !utf8.Valid(raw) {
		return "", "", false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	opening, e := d.Token()
	if e != nil || opening != json.Delim('{') {
		return "", "", false
	}
	seen := map[string]bool{}
	var next, expected string
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return "", "", false
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return "", "", false
		}
		seen[key] = true
		if key != "public_model_id" && key != "expected_public_model_id" {
			return "", "", false
		}
		var value string
		if d.Decode(&value) != nil {
			return "", "", false
		}
		if key == "public_model_id" {
			next = value
		} else {
			expected = value
		}
	}
	closing, e := d.Token()
	if e != nil || closing != json.Delim('}') || len(seen) != 2 {
		return "", "", false
	}
	if _, e = d.Token(); e != io.EOF {
		return "", "", false
	}
	return next, expected, publicModelNamePattern.MatchString(next) && publicModelNamePattern.MatchString(expected)
}
