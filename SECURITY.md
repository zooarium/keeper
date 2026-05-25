# Security Notes

Known issues to address before production hardening.

## Critical

### Hardcoded JWT Secret
- **File**: `pkg/config/config.go`, `config/config.yaml`
- **Issue**: JWT secret hardcoded as default value and committed to repo. Forged tokens possible if source is exposed.
- **Fix**: Remove default. Require `KEEPER_AUTH_JWT_SECRET` env var. Fail on boot if missing.

## High

### Hashed Password in API Responses
- **File**: `internal/user/model.go:15`
- **Issue**: `Password string \`json:"password,omitempty"\`` — bcrypt hash included in all user responses (`GET /users`, `GET /users/{id}`, auth response).
- **Fix**: Change tag to `json:"-"`.

### No Resource Ownership Check
- **File**: `internal/user/handler.go`
- **Issue**: `PUT /users/{id}`, `DELETE /users/{id}`, `GET /users/{id}` — any authenticated user can read/modify/delete any other user. JWT claims (`UserID`, `AppID`) never checked against path param.
- **Fix**: Validate `claims.AppID == user.AppID` (and optionally `claims.UserID == id`) in handler before delegating to service.

## Medium

### Internal Error Messages Leaked to Clients
- **File**: `internal/user/handler.go`, `internal/app/handler.go`
- **Issue**: `render.Error(w, http.StatusInternalServerError, err.Error())` — ent constraint errors, DB paths, and internal details returned to caller.
- **Fix**: Return generic `"internal server error"` to client. Keep `slog.Error` with full detail server-side only.

### CORS Wildcard + AllowCredentials
- **File**: `internal/platform/http/router.go:32`
- **Issue**: `AllowedOrigins: ["*"]` combined with `AllowCredentials: true`. Browsers block this per spec, but non-browser clients can exploit.
- **Fix**: Set explicit allowed origins when `AllowCredentials: true`. Never use `*` with credentials.

### No Request Body Size Limit
- **File**: All handlers using `json.NewDecoder(r.Body).Decode(&req)`
- **Issue**: No cap on request body size. Large payloads cause unbounded memory usage.
- **Fix**: Add `r.Body = http.MaxBytesReader(w, r.Body, 1<<20)` (1MB) before decoding.

## Low

### Weak JWT Claims
- **File**: `pkg/auth/jwt.go:30`
- **Issue**: Only `ExpiresAt` set. No `Issuer`, `Audience`, or `Subject`. Tokens are portable across services if secret is ever shared.
- **Fix**: Add `Issuer: "keeper"`, `Audience: ["keeper"]` to `RegisteredClaims`.

### Missing Index on app_id in kpr_user
- **File**: `ent/schema/user.go`
- **Issue**: `app_id` foreign key has no explicit index. Queries filtering by app_id do a full table scan.
- **Fix**: Add `Indexes()` method to User schema returning `index.Fields("app_id")`.

## Performance (Non-Security)

### No Pagination on List Endpoints
- `GET /users` and `GET /apps` return entire table. Add `limit`/`offset` query params.

### Double DB Round Trip on Create/Update
- `userRepository.Create` and `userRepository.Update` each fire a second `SELECT` to populate `AppName`. Acceptable now, revisit under load.
