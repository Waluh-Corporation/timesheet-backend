package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

func TestServer_Responses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := &Server{}

	// RespondSuccess
	wSuccess := httptest.NewRecorder()
	cSuccess, _ := gin.CreateTestContext(wSuccess)
	srv.RespondSuccess(cSuccess, http.StatusOK, gin.H{"item": "val"})
	if wSuccess.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", wSuccess.Code)
	}

	// RespondMessage
	wMsg := httptest.NewRecorder()
	cMsg, _ := gin.CreateTestContext(wMsg)
	srv.RespondMessage(cMsg, http.StatusAccepted, "accepted message")
	if wMsg.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", wMsg.Code)
	}

	// RespondDelete
	wDel := httptest.NewRecorder()
	cDel, _ := gin.CreateTestContext(wDel)
	srv.RespondDelete(cDel, http.StatusOK)
	if wDel.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", wDel.Code)
	}

	// RespondError
	wErr := httptest.NewRecorder()
	cErr, _ := gin.CreateTestContext(wErr)
	srv.RespondError(cErr, http.StatusBadRequest, "bad request")
	if wErr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", wErr.Code)
	}

	// RespondAbortError
	wAbort := httptest.NewRecorder()
	cAbort, _ := gin.CreateTestContext(wAbort)
	srv.RespondAbortError(cAbort, http.StatusForbidden, "forbidden abort")
	if wAbort.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", wAbort.Code)
	}
	if !cAbort.IsAborted() {
		t.Errorf("expected context to be aborted")
	}

	// RespondPaginated
	wPage := httptest.NewRecorder()
	cPage, _ := gin.CreateTestContext(wPage)
	srv.RespondPaginated(cPage, http.StatusOK, []string{"a", "b"}, 1, 10, 25)
	if wPage.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", wPage.Code)
	}
	var pageResp response.PaginatedResponse
	if err := json.Unmarshal(wPage.Body.Bytes(), &pageResp); err != nil {
		t.Fatalf("failed to decode paginated response: %v", err)
	}
	if pageResp.Pagination.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", pageResp.Pagination.TotalPages)
	}
}

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		RPDisplayName: "Test RP",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:3000"},
	}
	s, err := NewServer(nil, cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if s == nil || s.WebAuthn == nil {
		t.Fatal("expected non-nil server and webauthn")
	}
}

func TestServer_PublicBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := &Server{
		Cfg: &config.Config{
			FrontendURL: "http://localhost:3000/",
			RPOrigins:   []string{"https://portal.example.com", "http://insecure.example.com"},
		},
	}

	// 1. Fallback to FrontendURL when no proxy headers
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	if u := srv.publicBaseURL(c1); u != "http://localhost:3000" {
		t.Errorf("expected http://localhost:3000, got %s", u)
	}

	// 2. X-Forwarded-Host and X-Forwarded-Proto for trusted domain
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c2.Request.Header.Set("X-Forwarded-Host", "portal.example.com, internal.proxy")
	c2.Request.Header.Set("X-Forwarded-Proto", "https, http")
	if u := srv.publicBaseURL(c2); u != "https://portal.example.com" {
		t.Errorf("expected https://portal.example.com, got %s", u)
	}

	// 3. X-Forwarded-Host without Proto (non-TLS) for trusted domain
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c3.Request.Header.Set("X-Forwarded-Host", "insecure.example.com")
	if u := srv.publicBaseURL(c3); u != "http://insecure.example.com" {
		t.Errorf("expected http://insecure.example.com, got %s", u)
	}

	// 4. Untrusted host header poisoning attempt is rejected and falls back safely
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c4.Request.Header.Set("X-Forwarded-Host", "attacker.evil.com")
	c4.Request.Header.Set("X-Forwarded-Proto", "https")
	if u := srv.publicBaseURL(c4); u != "http://localhost:3000" {
		t.Errorf("expected fallback to http://localhost:3000, got %s", u)
	}
}

func TestServer_NilRedisClient(t *testing.T) {
	srv := &Server{RedisClient: nil}

	// putSession with nil client shouldn't panic
	data := &webauthn.SessionData{Challenge: "test"}
	srv.putSession("session-nil", data)

	// takeSession with nil client should return nil, false
	got, ok := srv.takeSession("session-nil")
	if ok || got != nil {
		t.Errorf("expected takeSession to fail when redis is nil, got %v, %v", got, ok)
	}

	// checkAndRecordResetCooldown with nil client should return true (graceful fallback)
	if !srv.checkAndRecordResetCooldown("user@example.com", "127.0.0.1", time.Minute) {
		t.Errorf("expected checkAndRecordResetCooldown to gracefully pass when redis is nil")
	}
}

func TestServer_Sessions(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	data := &webauthn.SessionData{
		Challenge: "test-challenge",
	}

	srv.putSession("session-1", data)

	got, ok := srv.takeSession("session-1")
	if !ok || got == nil || got.Challenge != "test-challenge" {
		t.Fatalf("takeSession failed: got %v, ok %v", got, ok)
	}

	// Subsequent take returns false (one-time use)
	_, ok2 := srv.takeSession("session-1")
	if ok2 {
		t.Errorf("expected session to be consumed on first take")
	}

	// Test non-existent session
	_, okMissing := srv.takeSession("non-existent-session")
	if okMissing {
		t.Errorf("expected non-existent session to be rejected")
	}

	// Test expired session in Redis (very short TTL)
	_ = rdb.Set(context.Background(), "webauthn:session:expired-session", `{"challenge":"expired"}`, 20*time.Millisecond).Err()
	time.Sleep(30 * time.Millisecond)
	_, okExpired := srv.takeSession("expired-session")
	if okExpired {
		t.Errorf("expected expired session to be rejected")
	}

	// Empty ID or nil data edge cases
	srv.putSession("", data)
	srv.putSession("nil-data", nil)
	_, okEmpty := srv.takeSession("")
	if okEmpty {
		t.Errorf("expected empty session id to return false")
	}
}

func TestServer_Sessions_RoundTripIntegrity(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	fullData := &webauthn.SessionData{
		Challenge:            "random-b64-challenge",
		UserID:               []byte("user-uuid-1234"),
		AllowedCredentialIDs: [][]byte{[]byte("cred-1"), []byte("cred-2")},
		UserVerification:     protocol.VerificationRequired,
		Expires:              time.Now().Add(5 * time.Minute).Truncate(time.Second),
	}

	sid := "full-session-id"
	srv.putSession(sid, fullData)

	retrieved, ok := srv.takeSession(sid)
	if !ok || retrieved == nil {
		t.Fatalf("expected round-trip retrieval to succeed")
	}
	if retrieved.Challenge != fullData.Challenge {
		t.Errorf("challenge mismatch: got %s, want %s", retrieved.Challenge, fullData.Challenge)
	}
	if string(retrieved.UserID) != string(fullData.UserID) {
		t.Errorf("userID mismatch: got %s, want %s", string(retrieved.UserID), string(fullData.UserID))
	}
	if len(retrieved.AllowedCredentialIDs) != 2 || string(retrieved.AllowedCredentialIDs[0]) != "cred-1" {
		t.Errorf("allowed credentials mismatch: got %v", retrieved.AllowedCredentialIDs)
	}
	if retrieved.UserVerification != fullData.UserVerification {
		t.Errorf("userVerification mismatch: got %s, want %s", retrieved.UserVerification, fullData.UserVerification)
	}
}

func TestServer_Sessions_ConcurrentTake(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	const sid = "concurrent-session-id"
	srv.putSession(sid, &webauthn.SessionData{Challenge: "concurrent-challenge"})

	const concurrency = 20
	var successCount atomic.Int32
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			if _, ok := srv.takeSession(sid); ok {
				successCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if successCount.Load() != 1 {
		t.Errorf("expected exactly 1 concurrent takeSession to succeed, got %d", successCount.Load())
	}
}

func TestServer_Sessions_LuaFallback(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// Test the fallback getdel script logic directly against existing and missing keys
	const getDelScript = `
		local v = redis.call('GET', KEYS[1])
		if not v or v == false then
			return nil
		end
		redis.call('DEL', KEYS[1])
		return v
	`
	ctx := context.Background()

	// 1. Missing key must return nil / redis.Nil, not boolean false or string "false"
	resMissing, errMissing := rdb.Eval(ctx, getDelScript, []string{"webauthn:session:nonexistent"}).Result()
	if !errors.Is(errMissing, redis.Nil) && resMissing != nil {
		t.Errorf("expected missing key to yield redis.Nil or nil result, got res=%v, err=%v", resMissing, errMissing)
	}

	// 2. Existing key returns the payload and deletes the key atomically
	_ = rdb.Set(ctx, "webauthn:session:existing", `{"challenge":"lua-test"}`, time.Minute).Err()
	resExisting, errExisting := rdb.Eval(ctx, getDelScript, []string{"webauthn:session:existing"}).Result()
	if errExisting != nil || resExisting != `{"challenge":"lua-test"}` {
		t.Errorf("expected existing payload, got res=%v, err=%v", resExisting, errExisting)
	}

	// 3. Second call on the same key must return nil / redis.Nil
	resConsumed, errConsumed := rdb.Eval(ctx, getDelScript, []string{"webauthn:session:existing"}).Result()
	if !errors.Is(errConsumed, redis.Nil) && resConsumed != nil {
		t.Errorf("expected consumed key to yield redis.Nil or nil result, got res=%v, err=%v", resConsumed, errConsumed)
	}
}

func TestServer_ResetCooldown(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	// 1. Zero or negative cooldown always returns true
	if !srv.checkAndRecordResetCooldown("user@example.com", "127.0.0.1", 0) {
		t.Errorf("expected cooldown <= 0 to pass")
	}
	if !srv.checkAndRecordResetCooldown("user@example.com", "127.0.0.1", -1*time.Second) {
		t.Errorf("expected cooldown < 0 to pass")
	}

	// 2. Empty email and IP returns true
	if !srv.checkAndRecordResetCooldown("", "", 10*time.Second) {
		t.Errorf("expected empty email and IP to pass")
	}

	// 3. Normal email cooldown flow (SET NX EX)
	email := fmt.Sprintf("test_cooldown_%d@example.com", time.Now().UnixNano())
	cooldown := 200 * time.Millisecond

	// First request succeeds
	if !srv.checkAndRecordResetCooldown(email, "10.0.0.1", cooldown) {
		t.Errorf("expected first reset request to pass")
	}

	// Second request within cooldown is throttled
	if srv.checkAndRecordResetCooldown(email, "10.0.0.1", cooldown) {
		t.Errorf("expected consecutive reset request within cooldown to be throttled")
	}

	// Wait past cooldown
	time.Sleep(250 * time.Millisecond)

	// Third request after cooldown succeeds
	if !srv.checkAndRecordResetCooldown(email, "10.0.0.1", cooldown) {
		t.Errorf("expected reset request after cooldown expiration to pass")
	}

	// 4. IP rate limit check (max 5 requests per window)
	ip := fmt.Sprintf("192.168.1.%d", time.Now().UnixNano()%250+1)
	for i := 1; i <= 5; i++ {
		uniqueEmail := fmt.Sprintf("ip_test_%d_%d@example.com", time.Now().UnixNano(), i)
		if !srv.checkAndRecordResetCooldown(uniqueEmail, ip, 1*time.Second) {
			t.Fatalf("expected request %d within IP quota to succeed", i)
		}
	}

	// 6th request from same IP should be rate limited even with different email
	sixthEmail := fmt.Sprintf("ip_test_sixth_%d@example.com", time.Now().UnixNano())
	if srv.checkAndRecordResetCooldown(sixthEmail, ip, 1*time.Second) {
		t.Errorf("expected 6th request to be blocked by IP rate limit")
	}
}

func TestServer_ResetCooldown_IPWithPort(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	const baseIP = "192.0.2.77"
	for i := 1; i <= 5; i++ {
		email := fmt.Sprintf("port_user_%d@example.com", i)
		ipWithPort := fmt.Sprintf("%s:%d", baseIP, 50000+i)
		if !srv.checkAndRecordResetCooldown(email, ipWithPort, time.Minute) {
			t.Fatalf("expected request %d with port to succeed", i)
		}
	}

	// 6th request from same IP with different port should be throttled
	blocked := srv.checkAndRecordResetCooldown("port_user_6@example.com", fmt.Sprintf("%s:59999", baseIP), time.Minute)
	if blocked {
		t.Errorf("expected 6th request from same IP regardless of port to be throttled")
	}
}

func TestServer_ResetCooldown_ConcurrentThrottling(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	email := "concurrent_cooldown@example.com"
	const concurrency = 20
	var successCount atomic.Int32
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			ip := fmt.Sprintf("10.10.10.%d", workerID+1)
			if srv.checkAndRecordResetCooldown(email, ip, 2*time.Second) {
				successCount.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if successCount.Load() != 1 {
		t.Errorf("expected exactly 1 concurrent reset cooldown request to succeed, got %d", successCount.Load())
	}
}

func TestServer_ResetCooldown_ConcurrentIPQuotaRollback(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	const ip = "198.51.100.99"
	const concurrency = 20
	var successCount atomic.Int32
	var failedEmails sync.Map
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			e := fmt.Sprintf("ip_concurrency_%d@example.com", id)
			if srv.checkAndRecordResetCooldown(e, ip, 5*time.Second) {
				successCount.Add(1)
			} else {
				failedEmails.Store(e, true)
			}
		}(i)
	}
	wg.Wait()

	if successCount.Load() != 5 {
		t.Errorf("expected exactly 5 requests to succeed within IP quota, got %d", successCount.Load())
	}

	// Check that the failed emails had their email reservation rolled back:
	// If tested with a fresh, unthrottled IP, they should succeed!
	var rollbackChecked int
	failedEmails.Range(func(key, _ interface{}) bool {
		email := key.(string)
		freshIP := fmt.Sprintf("203.0.113.%d", rollbackChecked+1)
		if !srv.checkAndRecordResetCooldown(email, freshIP, 5*time.Second) {
			t.Errorf("expected rolled back email %s to succeed from fresh IP", email)
		}
		rollbackChecked++
		return true
	})
	if rollbackChecked != 15 {
		t.Errorf("expected 15 rolled back emails, verified %d", rollbackChecked)
	}
}

func TestServer_ResetCooldown_UnreachableRedis(t *testing.T) {
	// Point to an unreachable address with a very quick dial timeout
	rdb := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:65534",
		DialTimeout: 20 * time.Millisecond,
		ReadTimeout: 20 * time.Millisecond,
	})
	srv := &Server{RedisClient: rdb}

	// When Redis is unreachable, checkAndRecordResetCooldown should log a warning
	// and return true (gracefully failing open on the fast-path so DB cooldown can take over)
	if !srv.checkAndRecordResetCooldown("unreachable@example.com", "127.0.0.1", time.Minute) {
		t.Errorf("expected unreachable redis to gracefully bypass fast-path, got false")
	}
}

func TestServer_ResetCooldown_IPCanonicalization(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	// 1. Test comma-separated proxy chain
	const proxyIP1 = "203.0.113.195, 70.41.3.18"
	const proxyIP2 = "203.0.113.195, 10.0.0.1, 192.168.1.1"
	for i := 1; i <= 3; i++ {
		if !srv.checkAndRecordResetCooldown(fmt.Sprintf("proxy1_%d@example.com", i), proxyIP1, time.Minute) {
			t.Fatalf("expected request %d with proxy chain to succeed", i)
		}
	}
	for i := 4; i <= 5; i++ {
		if !srv.checkAndRecordResetCooldown(fmt.Sprintf("proxy2_%d@example.com", i), proxyIP2, time.Minute) {
			t.Fatalf("expected request %d with proxy chain 2 to succeed", i)
		}
	}
	// 6th request with raw IP should be blocked because the first IP in proxy chain was canonicalized
	if srv.checkAndRecordResetCooldown("proxy_blocked@example.com", "203.0.113.195", time.Minute) {
		t.Errorf("expected 6th request from canonicalized proxy IP to be blocked")
	}

	// 2. Test IPv6 formatting variations: brackets, port, zone specifier (%eth0), compressed vs uncompressed
	const (
		ipv6Raw          = "2001:db8::1"
		ipv6WithBrackets = "[2001:db8::1]"
		ipv6WithPort     = "[2001:db8::1]:8443"
		ipv6WithZone     = "2001:db8::1%eth0"
		ipv6ZoneAndPort  = "[2001:db8::1%eth0]:8443"
		ipv6Uncompressed = "2001:0db8:0000:0000:0000:0000:0000:0001"
	)

	// Consume 5 slots with different notations of the same IPv6 address
	if !srv.checkAndRecordResetCooldown("v6_1@example.com", ipv6Raw, time.Minute) {
		t.Errorf("expected v6_1 to succeed")
	}
	if !srv.checkAndRecordResetCooldown("v6_2@example.com", ipv6WithBrackets, time.Minute) {
		t.Errorf("expected v6_2 to succeed")
	}
	if !srv.checkAndRecordResetCooldown("v6_3@example.com", ipv6WithPort, time.Minute) {
		t.Errorf("expected v6_3 to succeed")
	}
	if !srv.checkAndRecordResetCooldown("v6_4@example.com", ipv6WithZone, time.Minute) {
		t.Errorf("expected v6_4 to succeed")
	}
	if !srv.checkAndRecordResetCooldown("v6_5@example.com", ipv6ZoneAndPort, time.Minute) {
		t.Errorf("expected v6_5 to succeed")
	}
	// 6th request with uncompressed notation should be blocked
	if srv.checkAndRecordResetCooldown("v6_6@example.com", ipv6Uncompressed, time.Minute) {
		t.Errorf("expected 6th request from uncompressed IPv6 to be blocked by shared rate limit")
	}
}

func TestServer_ResetCooldown_SubMillisecondCooldown(t *testing.T) {
	mr := miniredis.RunT(t)
	// Do not use ClockTTL(true) so time does not elapse between network calls
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	// Sub-millisecond cooldown (e.g. 500 microseconds) should be safely clamped to at least 1ms
	// preventing go-redis zero truncation and premature deletion
	email := "submilli@example.com"
	ip := "192.0.2.123"
	subCooldown := 500 * time.Microsecond

	// First request succeeds
	if !srv.checkAndRecordResetCooldown(email, ip, subCooldown) {
		t.Fatalf("expected first sub-millisecond request to succeed")
	}

	// Immediate second request within cooldown should be throttled
	if srv.checkAndRecordResetCooldown(email, ip, subCooldown) {
		t.Errorf("expected immediate second request within cooldown to be throttled")
	}

	// Fast forward time past the 1ms clamped cooldown
	mr.FastForward(5 * time.Millisecond)

	// Third request after expiration should succeed
	if !srv.checkAndRecordResetCooldown(email, ip, subCooldown) {
		t.Errorf("expected request after clamped cooldown expiration to succeed")
	}
}

func TestServer_Sessions_ContextCancellation(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.ClockTTL(true)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	srv := &Server{RedisClient: rdb}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// takeSessionCtx with canceled context should return false without panic
	got, ok := srv.takeSessionCtx(canceledCtx, "canceled-sid")
	if ok || got != nil {
		t.Errorf("expected takeSessionCtx with canceled context to return false, got %v, %v", got, ok)
	}

	// putSessionCtx with canceled context should not panic
	srv.putSessionCtx(canceledCtx, "canceled-sid", &webauthn.SessionData{Challenge: "challenge"})

	// checkAndRecordResetCooldownCtx with canceled context should gracefully bypass without panic
	if !srv.checkAndRecordResetCooldownCtx(canceledCtx, "canceled@example.com", "127.0.0.1", time.Minute) {
		t.Errorf("expected checkAndRecordResetCooldownCtx with canceled context to bypass gracefully")
	}
}

func TestServer_Middlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authSvc := auth.NewService("test-secret-at-least-32-chars-long!", 24*time.Hour)
	srv := &Server{
		Auth: authSvc,
	}

	r := gin.New()
	r.Use(CORSMiddleware())
	r.OPTIONS("/cors", func(c *gin.Context) {})

	// Test CORS OPTIONS preflight without Origin
	reqOpts := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	wOpts := httptest.NewRecorder()
	r.ServeHTTP(wOpts, reqOpts)
	if wOpts.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", wOpts.Code)
	}
	if wOpts.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *")
	}

	// Test CORS allowed origin (e.g. localhost:3000 in dev)
	reqAllowed := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	reqAllowed.Header.Set("Origin", "http://localhost:3000")
	wAllowed := httptest.NewRecorder()
	r.ServeHTTP(wAllowed, reqAllowed)
	if wAllowed.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin to match allowed origin")
	}
	if wAllowed.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials: true for allowed origin")
	}

	// Test CORS untrusted origin (must not be allowed with credentials)
	reqUntrusted := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	reqUntrusted.Header.Set("Origin", "http://attacker.evil.com")
	wUntrusted := httptest.NewRecorder()
	r.ServeHTTP(wUntrusted, reqUntrusted)
	if wUntrusted.Header().Get("Access-Control-Allow-Credentials") == "true" {
		t.Errorf("untrusted origin must not receive Access-Control-Allow-Credentials: true")
	}
	if wUntrusted.Header().Get("Access-Control-Allow-Origin") == "http://attacker.evil.com" {
		t.Errorf("untrusted origin must not be reflected in Access-Control-Allow-Origin")
	}

	// Test AuthMiddleware & AdminOnly
	r.GET("/protected", srv.AuthMiddleware(), func(c *gin.Context) {
		uid := currentUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": uid})
	})
	r.GET("/admin-only", srv.AuthMiddleware(), srv.AdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "admin"})
	})

	// Missing token
	reqNoToken := httptest.NewRequest(http.MethodGet, "/protected", nil)
	wNoToken := httptest.NewRecorder()
	r.ServeHTTP(wNoToken, reqNoToken)
	if wNoToken.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on missing token, got %d", wNoToken.Code)
	}

	// Invalid token
	reqBadToken := httptest.NewRequest(http.MethodGet, "/protected", nil)
	reqBadToken.Header.Set("Authorization", "Bearer invalid-token")
	wBadToken := httptest.NewRecorder()
	r.ServeHTTP(wBadToken, reqBadToken)
	if wBadToken.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on invalid token, got %d", wBadToken.Code)
	}

	// Valid user token
	userToken, err := authSvc.GenerateToken(&models.User{ID: 42, Username: "testuser", Role: models.RoleUser})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	reqUser := httptest.NewRequest(http.MethodGet, "/protected", nil)
	reqUser.Header.Set("Authorization", "Bearer "+userToken)
	wUser := httptest.NewRecorder()
	r.ServeHTTP(wUser, reqUser)
	if wUser.Code != http.StatusOK {
		t.Errorf("expected 200 with valid user token, got %d", wUser.Code)
	}

	// Valid user token trying admin-only route -> 403
	reqUserAdmin := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	reqUserAdmin.Header.Set("Authorization", "Bearer "+userToken)
	wUserAdmin := httptest.NewRecorder()
	r.ServeHTTP(wUserAdmin, reqUserAdmin)
	if wUserAdmin.Code != http.StatusForbidden {
		t.Errorf("expected 403 for user accessing admin route, got %d", wUserAdmin.Code)
	}

	// Valid admin token
	adminToken, err := authSvc.GenerateToken(&models.User{ID: 1, Username: "admin", Role: models.RoleAdmin})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	wAdmin := httptest.NewRecorder()
	r.ServeHTTP(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 for admin accessing admin route, got %d", wAdmin.Code)
	}
}

func TestServer_CORSMiddleware_CORSAllowedOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("GIN_MODE", "release")

	srv := &Server{
		Cfg: &config.Config{
			FrontendURL: "https://timesheet-frontend-dev.fafr.my.id",
			CORSAllowedOrigins: []string{
				"http://localhost:3000",
				"http://127.0.0.1:5173",
			},
		},
	}

	r := gin.New()
	r.Use(srv.CORSMiddleware())
	r.OPTIONS("/cors", func(c *gin.Context) {})

	// In release mode, origin listed in CORSAllowedOrigins must be allowed
	reqLocalhost := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	reqLocalhost.Header.Set("Origin", "http://localhost:3000")
	wLocalhost := httptest.NewRecorder()
	r.ServeHTTP(wLocalhost, reqLocalhost)
	if wLocalhost.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin to be reflected for CORSAllowedOrigins in release mode")
	}
	if wLocalhost.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials: true for CORSAllowedOrigins")
	}

	// FrontendURL must also be allowed
	reqFrontend := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	reqFrontend.Header.Set("Origin", "https://timesheet-frontend-dev.fafr.my.id")
	wFrontend := httptest.NewRecorder()
	r.ServeHTTP(wFrontend, reqFrontend)
	if wFrontend.Header().Get("Access-Control-Allow-Origin") != "https://timesheet-frontend-dev.fafr.my.id" {
		t.Errorf("expected Access-Control-Allow-Origin for FrontendURL")
	}

	// Origin not in CORSAllowedOrigins or FrontendURL must be denied in release mode
	reqEvil := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	reqEvil.Header.Set("Origin", "http://attacker.com")
	wEvil := httptest.NewRecorder()
	r.ServeHTTP(wEvil, reqEvil)
	if wEvil.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("untrusted origin must not receive Access-Control-Allow-Origin in release mode")
	}

	// Test wildcard in CORSAllowedOrigins
	srvWildcard := &Server{
		Cfg: &config.Config{
			CORSAllowedOrigins: []string{"*"},
		},
	}
	rWildcard := gin.New()
	rWildcard.Use(srvWildcard.CORSMiddleware())
	rWildcard.OPTIONS("/cors", func(c *gin.Context) {})

	reqAny := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	reqAny.Header.Set("Origin", "https://any-domain.example.com")
	wAny := httptest.NewRecorder()
	rWildcard.ServeHTTP(wAny, reqAny)
	if wAny.Header().Get("Access-Control-Allow-Origin") != "https://any-domain.example.com" {
		t.Errorf("expected wildcard CORS to allow any origin")
	}

	// isTrustedHost test with CORSAllowedOrigins
	if !srv.isTrustedHost("localhost:3000") {
		t.Errorf("expected localhost:3000 to be trusted host via CORSAllowedOrigins")
	}
	if !srv.isTrustedHost("127.0.0.1:5173") {
		t.Errorf("expected 127.0.0.1:5173 to be trusted host via CORSAllowedOrigins")
	}
}
