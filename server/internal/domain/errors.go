package domain

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Code is a stable, user-facing error identifier shared by the API and the client.
type Code string

const (
	CodeInvalidTicker       Code = "INVALID_TICKER"
	CodeDuplicateTicker     Code = "DUPLICATE_TICKER"
	CodeUnknownSymbol       Code = "UNKNOWN_SYMBOL"
	CodeNotInWatchlist      Code = "NOT_IN_WATCHLIST"
	CodeInvalidRange        Code = "INVALID_RANGE"
	CodeMissingAPIKey       Code = "MISSING_API_KEY"
	CodeProviderUnavailable Code = "PROVIDER_UNAVAILABLE"
	CodeRateLimited         Code = "RATE_LIMITED"
	CodeChartUnavailable    Code = "CHART_UNAVAILABLE"
	CodeInternalError       Code = "INTERNAL_ERROR"
	CodeNotFound            Code = "NOT_FOUND"
	CodeRequestTooLarge     Code = "REQUEST_TOO_LARGE"

	// Authentication extension codes (docs/contracts-auth.md section 7).
	CodeUnauthenticated     Code = "UNAUTHENTICATED"
	CodeInvalidCredentials  Code = "INVALID_CREDENTIALS"
	CodeInvalidCode         Code = "INVALID_CODE"
	CodeInvalidRecoveryCode Code = "INVALID_RECOVERY_CODE"
	CodeChallengeExpired    Code = "CHALLENGE_EXPIRED"
	CodeEnrollmentExpired   Code = "ENROLLMENT_EXPIRED"
	CodeUsernameTaken       Code = "USERNAME_TAKEN"
	CodeInvalidUsername     Code = "INVALID_USERNAME"
	CodeWeakPassword        Code = "WEAK_PASSWORD"
	CodeInvalidRequest      Code = "INVALID_REQUEST"
	CodeCrossOriginRequest  Code = "CROSS_ORIGIN_REQUEST"
)

// Error is a domain failure with a stable code and user-visible copy.
type Error struct {
	Code    Code
	Message string
	// RetryAfter is set for RATE_LIMITED so the transport can send Retry-After.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewError constructs a domain error.
func NewError(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// NewRateLimited builds the 429 error with a whole-second retry window.
func NewRateLimited(retryAfter time.Duration) *Error {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return &Error{
		Code:       CodeRateLimited,
		Message:    TooManyRequests(seconds),
		RetryAfter: time.Duration(seconds) * time.Second,
	}
}

// IsCode reports whether err is (or wraps) a domain error with the given code.
func IsCode(err error, code Code) bool {
	var domainError *Error
	return errors.As(err, &domainError) && domainError.Code == code
}

// DefaultMessage returns the exact user-visible copy for a code. Codes whose copy
// interpolates a ticker (DUPLICATE_TICKER, NOT_IN_WATCHLIST) have no fixed message;
// their owners build one with the helpers below.
func DefaultMessage(code Code) string {
	switch code {
	case CodeInvalidTicker:
		return "Enter a valid ticker, e.g. AAPL."
	case CodeUnknownSymbol:
		return "Symbol not found."
	case CodeInvalidRange:
		return "Range must be 1M, 3M or 1Y."
	case CodeMissingAPIKey:
		return "Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY."
	case CodeProviderUnavailable:
		return "Data provider is unavailable. Try again shortly."
	case CodeRateLimited:
		return TooManyRequests(1)
	case CodeChartUnavailable:
		return "Chart unavailable"
	case CodeInternalError:
		return "Something went wrong. Try again shortly."
	case CodeNotFound:
		return "Not found."
	case CodeRequestTooLarge:
		return "Request body is too large."
	case CodeUnauthenticated:
		return "Authentication required."
	case CodeInvalidCredentials:
		return "Invalid username or password."
	case CodeInvalidCode:
		return "Invalid verification code."
	case CodeInvalidRecoveryCode:
		return "Invalid recovery code."
	case CodeChallengeExpired:
		return "Your sign-in attempt expired. Sign in again."
	case CodeEnrollmentExpired:
		return "Setup timed out. Start again."
	case CodeUsernameTaken:
		return "That username is already taken."
	case CodeInvalidUsername:
		return "Use 3-32 lowercase letters, numbers, dots, dashes or underscores."
	case CodeWeakPassword:
		return "Use at least 12 characters."
	case CodeInvalidRequest:
		return "Enter your username and password."
	case CodeCrossOriginRequest:
		return "Cross-origin requests are not allowed."
	default:
		return "Something went wrong. Try again shortly."
	}
}

// DuplicateMessage is the copy for adding an already-listed ticker.
func DuplicateMessage(ticker Ticker) string {
	return fmt.Sprintf("%s is already in your watchlist.", ticker)
}

// NotInWatchlistMessage is the copy for removing a ticker that is not listed.
func NotInWatchlistMessage(ticker Ticker) string {
	return fmt.Sprintf("%s is not in your watchlist.", ticker)
}

// TooManyRequests is the rate-limit copy for a whole number of seconds. A
// one-second window uses the singular, so no response ever says "1 seconds".
func TooManyRequests(seconds int) string {
	if seconds < 1 {
		seconds = 1
	}
	if seconds == 1 {
		return "Too many requests. Try again in 1 second."
	}
	return fmt.Sprintf("Too many requests. Try again in %d seconds.", seconds)
}
