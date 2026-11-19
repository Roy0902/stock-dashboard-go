package api

import (
	"net/http"

	"stock-dashboard/server/internal/domain"
)

const maxAddBodyBytes = 4 << 10

func (h *handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusBody{Status: "ok"})
}

func (h *handlers) listWatchlist(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toWatchlistBody(h.deps.Watchlist.List(sessionUserID(r.Context()))))
}

func (h *handlers) addWatchlist(w http.ResponseWriter, r *http.Request) {
	raw, err := decodeAddRequest(w, r)
	if err != nil {
		h.fail(w, r, err)
		return
	}

	ticker, err := domain.Canonicalize(raw)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	userID := sessionUserID(r.Context())

	// Fast duplicate check without holding the store lock. It is repeated inside
	// Update, so a concurrent writer cannot win between the check and the write.
	if h.deps.Watchlist.List(userID).Contains(ticker) {
		h.fail(w, r, domain.NewError(domain.CodeDuplicateTicker, domain.DuplicateMessage(ticker)))
		return
	}

	// Verification is a network call: it must not run while the store is locked.
	if err := h.deps.Quotes.VerifySymbol(r.Context(), ticker); err != nil {
		h.fail(w, r, mapProviderFailure(err, domain.CodeProviderUnavailable))
		return
	}

	watchlist, err := h.deps.Watchlist.Update(userID, func(current domain.Watchlist) (domain.Watchlist, error) {
		if current.Contains(ticker) {
			return domain.Watchlist{}, domain.NewError(domain.CodeDuplicateTicker, domain.DuplicateMessage(ticker))
		}
		return current.Add(ticker.String())
	})
	if err != nil {
		h.fail(w, r, mapProviderFailure(err, domain.CodeProviderUnavailable))
		return
	}
	writeJSON(w, http.StatusCreated, toWatchlistBody(watchlist))
}

func (h *handlers) removeWatchlist(w http.ResponseWriter, r *http.Request) {
	ticker, err := domain.Canonicalize(r.PathValue("ticker"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	userID := sessionUserID(r.Context())
	watchlist, err := h.deps.Watchlist.Update(userID, func(current domain.Watchlist) (domain.Watchlist, error) {
		return current.Remove(ticker.String())
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.deps.Quotes.Forget(ticker)
	writeJSON(w, http.StatusOK, toWatchlistBody(watchlist))
}

func (h *handlers) quotes(w http.ResponseWriter, r *http.Request) {
	if !h.deps.Keys.Complete() {
		h.fail(w, r, domainError(domain.CodeMissingAPIKey))
		return
	}

	tickers := h.deps.Watchlist.List(sessionUserID(r.Context())).Tickers()
	batch := h.deps.Quotes.Quotes(r.Context(), tickers)
	if len(batch.Quotes) == 0 && len(tickers) > 0 {
		if batch.Canceled {
			// A cancelled refresh is not the documented empty watchlist. The
			// response may never reach the client, but it is never a clean 200.
			h.fail(w, r, domainError(domain.CodeProviderUnavailable))
			return
		}
		if batch.RateLimitedAfter != nil {
			h.fail(w, r, rateLimitedError(*batch.RateLimitedAfter))
			return
		}
		if batch.CredentialFailure {
			// Upstream rejected the key (401/403) or a key is absent: the
			// actionable copy is the same MISSING_API_KEY envelope.
			h.fail(w, r, domainError(domain.CodeMissingAPIKey))
			return
		}
		if batch.Unavailable {
			h.fail(w, r, domainError(domain.CodeProviderUnavailable))
			return
		}
	}
	writeJSON(w, http.StatusOK, toQuotesBody(batch.Quotes))
}

func (h *handlers) chart(w http.ResponseWriter, r *http.Request) {
	ticker, err := domain.Canonicalize(r.PathValue("ticker"))
	if err != nil {
		h.fail(w, r, err)
		return
	}

	rangeValue, err := chartRange(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}

	if !h.deps.Keys.Complete() {
		h.fail(w, r, domainError(domain.CodeMissingAPIKey))
		return
	}

	points, err := h.deps.Quotes.DailySeries(r.Context(), ticker, rangeValue)
	if err != nil {
		h.fail(w, r, mapProviderFailure(err, domain.CodeChartUnavailable))
		return
	}
	writeJSON(w, http.StatusOK, toChartBody(ticker, rangeValue, points))
}

func (h *handlers) apiNotFound(w http.ResponseWriter, r *http.Request) {
	h.fail(w, r, domainError(domain.CodeNotFound))
}

func chartRange(r *http.Request) (domain.PriceRange, error) {
	values, present := r.URL.Query()["range"]
	if !present {
		return domain.Range1M, nil
	}
	return domain.ParsePriceRange(values[0])
}

// decodeAddRequest enforces the shared body rules with the ticker-specific copy.
func decodeAddRequest(w http.ResponseWriter, r *http.Request) (string, error) {
	var request struct {
		Ticker string `json:"ticker"`
	}
	if err := decodeJSON(w, r, &request, maxAddBodyBytes, func() error { return invalidTickerError() }); err != nil {
		return "", err
	}
	return request.Ticker, nil
}
