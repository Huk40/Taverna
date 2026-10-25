package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

// Decode accepts exactly one JSON value and bounds memory used by HTTP input.
func Decode(w http.ResponseWriter, r *http.Request, dst any, limit int64, allowEmpty bool) bool {
	defer r.Body.Close()
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || typ != "application/json" {
		Error(w, http.StatusUnsupportedMediaType, "Ожидается application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err = dec.Decode(dst)
	if allowEmpty && errors.Is(err, io.EOF) {
		return true
	}
	if err == nil {
		if err = dec.Decode(new(any)); errors.Is(err, io.EOF) {
			return true
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		Error(w, http.StatusRequestEntityTooLarge, "Размер запроса превышает лимит")
	} else {
		Error(w, http.StatusBadRequest, "Ожидается один корректный JSON-объект")
	}
	return false
}

func Error(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func UUID(value string) bool {
	var id pgtype.UUID
	return len(value) == 36 && id.Scan(value) == nil && id.Valid
}

// Page preserves the existing array response. Clients follow X-Next-Offset.
type Page struct{ Limit, Offset int }

func Pagination(w http.ResponseWriter, r *http.Request) (Page, bool) {
	p := Page{Limit: 50}
	for key, dest := range map[string]*int{"limit": &p.Limit, "offset": &p.Offset} {
		if value := r.URL.Query().Get(key); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 || n > 1_000_000 || (key == "limit" && (n < 1 || n > 100)) {
				Error(w, 400, "Некорректные параметры страницы")
				return Page{}, false
			}
			*dest = n
		}
	}
	return p, true
}

func NextPage(w http.ResponseWriter, p Page, count int) {
	if count == p.Limit {
		w.Header().Set("X-Next-Offset", strconv.Itoa(p.Offset+count))
	}
}
