package internal

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var peerCfg = &Config{Debug: true, requestTimeout: 100 * time.Millisecond}

func newNoCallbackExpectedServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Fatal("expected callback URL not being called")
	}))
}

func newCallbackExpectedServerWithResponse(t *testing.T, reply func(w http.ResponseWriter)) (*httptest.Server, func()) {
	called := make(chan bool, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/info", req.RequestURI, "callback URI mismatch")
		reply(w)
		called <- true
	}))
	cleanup := func() {
		server.Close()
		select {
		case <-called:
			return
		default:
			t.Fatal("expected callback URL being called, but was not")
		}
	}
	return server, cleanup
}

func newCallbackExpectedServerWithNewPodInfo(t *testing.T) (*httptest.Server, func()) {
	response, err := json.Marshal(Peer{
		Name:            "new name",
		Description:     "new description",
		SoftwareVersion: "0.9001.23@7654321",
	})
	require.NoError(t, err, "marshalling pod info for callback failed")
	return newCallbackExpectedServerWithResponse(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(response)
	})
}

func randomPort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if listener, err = net.Listen("tcp6", "[::1]:0"); err != nil {
			t.Fatalf("failed to listen on a port: %v", err)
			return 0
		}
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func newUserProfile(username string) types.Profile {
	return types.Profile{
		Type:     "user",
		Username: username,
	}
}
func newRequestWithUA(ua string) *http.Request {
	req, err := http.NewRequest("GET", "http://localhost/user/foo/twtxt.txt", nil)
	if err != nil {
		panic("creating test HTTP request failed")
	}
	req.Header.Set("User-Agent", ua)
	return req
}

func newCacheWithPodInfo(podBaseURL string, lastSeenAndUpdated time.Time) *Cache {
	cache := NewCache(peerCfg)
	cache.Peers[podBaseURL] = &Peer{
		Name:            "old name",
		Description:     "old description",
		SoftwareVersion: "0.42.0@1234567",
		LastSeen:        lastSeenAndUpdated,
		LastUpdated:     lastSeenAndUpdated,
	}
	return cache
}

func assertAfterOrAt(t *testing.T, expected, actual time.Time, msg string) {
	if actual.Before(actual) {
		t.Fatalf("%s is not after or at %s: %s", actual.Format(time.RFC3339), expected.Format(time.RFC3339), msg)
	}
}

func assertBefore(t *testing.T, expected, actual time.Time, msg string) {
	if !actual.Before(expected) {
		t.Fatalf("%s is not before %s: %s", actual.Format(time.RFC3339), expected.Format(time.RFC3339), msg)
	}
}

func assertPodInfoNotInserted(t *testing.T, cache *Cache) {
	assert.Empty(t, cache.Peers, "cached peers should not have been updated")
}

func assertPodInfoNotUpdatedExceptLastSeen(t *testing.T, cache *Cache, podBaseURL string,
	expectedNowReferenceBeforeCallForLastSeen, expectedLastUpdated time.Time) {

	podInfo, ok := cache.Peers[podBaseURL]
	require.True(t, ok, "cached pod info should not have been removed from the cache")
	require.NotNil(t, podInfo, "cached pod info should not have been removed from the cache")
	assert.Equal(t, "old name", podInfo.Name, "cached pod name should not have been updated")
	assert.Equal(t, "old description", podInfo.Description, "cached pod description shold not have been updated")
	assert.Equal(t, "0.42.0@1234567", podInfo.SoftwareVersion, "cached pod software version should not have been updated")
	assertAfterOrAt(t, expectedNowReferenceBeforeCallForLastSeen, podInfo.LastSeen,
		"cached last seen should have been updated to current point in time")
	assertBefore(t, expectedNowReferenceBeforeCallForLastSeen.Add(10*time.Second), /* allow for a little clock skew */
		podInfo.LastSeen, "cached last seen should not have been updated to be in the future")
	assert.Equal(t, expectedLastUpdated, podInfo.LastUpdated, "cached pod last updated should not have been updated")
}

func assertPodInfoUpdated(t *testing.T, cache *Cache, podBaseURL string,
	expectedNowReferenceBeforeCallForLastSeenAndUpdated time.Time) {

	podInfo, ok := cache.Peers[podBaseURL]
	require.True(t, ok, "cached pod info should have been inserted into/not removed from the cache")
	require.NotNil(t, podInfo, "cached pod info should have been inserted into/not removed from the cache")
	assert.Equal(t, "new name", podInfo.Name, "cached pod name should have been updated")
	assert.Equal(t, "new description", podInfo.Description, "cached pod description shold have been updated")
	assert.Equal(t, "0.9001.23@7654321", podInfo.SoftwareVersion, "cached pod software version should have been updated")
	assertAfterOrAt(t, expectedNowReferenceBeforeCallForLastSeenAndUpdated, podInfo.LastSeen,
		"cached last seen should have been updated to current point in time")
	assertBefore(t, expectedNowReferenceBeforeCallForLastSeenAndUpdated.Add(10*time.Second), /* allow for a little clock skew */
		podInfo.LastSeen, "cached last seen should not have been updated to be in the future")
	assertAfterOrAt(t, expectedNowReferenceBeforeCallForLastSeenAndUpdated, podInfo.LastUpdated,
		"cached last updated should have been updated to current point in time")
	assertBefore(t, expectedNowReferenceBeforeCallForLastSeenAndUpdated.Add(10*time.Second), /* allow for a little clock skew */
		podInfo.LastUpdated, "cached last updated should not have been updated to be in the future")
}

func TestCache_DetectPodFromRequest_whenNonTwtxtUserAgent_thenDoNothing(t *testing.T) {
	server := newNoCallbackExpectedServer(t)
	defer server.Close()

	cache := NewCache(peerCfg)
	profile := newUserProfile("bob")
	req := newRequestWithUA("Linguee Bot (http://www.linguee.com/bot; bot@linguee.com)")

	assert.NoError(t, cache.DetectClientFromRequest(req, profile), "detecting pod failed")
	assertPodInfoNotInserted(t, cache)
}

func TestCache_DetectPodFromRequest_whenNonYarnTwtxtUserAgent_thenDoNothing(t *testing.T) {
	server := newNoCallbackExpectedServer(t)
	defer server.Close()

	cache := NewCache(peerCfg)
	profile := newUserProfile("bob")
	req := newRequestWithUA("twtxt/1.2.3 (+https://example.com/twtxt.txt; @foo)")

	assert.NoError(t, cache.DetectClientFromRequest(req, profile), "detecting pod failed")
	assertPodInfoNotInserted(t, cache)
}

func TestCache_DetectPodFromRequest_whenPodAlreadySeenWithinConfiguredTTL_thenDoNotCallbackButUpdateLastSeen(t *testing.T) {
	server := newNoCallbackExpectedServer(t)
	defer server.Close()

	lastSeenAndUpdated := time.Now().Add(-3 * time.Minute)
	cache := newCacheWithPodInfo(server.URL, lastSeenAndUpdated)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.42.0@1234567 (+%s/user/bar/twtxt.txt; @bar)", server.URL))
	now := time.Now()

	assert.NoError(t, cache.DetectClientFromRequest(req, profile), "detecting pod failed")
	assertPodInfoNotUpdatedExceptLastSeen(t, cache, server.URL, now, lastSeenAndUpdated)
}

func TestCache_DetectPodFromRequest_whenPodNeverSeen_thenCallbackAndPopulateCache(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithNewPodInfo(t)
	defer cleanup()

	cache := NewCache(peerCfg)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))
	now := time.Now()

	assert.NoError(t, cache.DetectClientFromRequest(req, profile), "detecting pod failed")
	assertPodInfoUpdated(t, cache, server.URL, now)
}

func TestCache_DetectPodFromRequest_whenPodAlreadySeenOutsideConfiguredTTL_thenCallbackAndUpdateCache(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithNewPodInfo(t)
	defer cleanup()

	lastSeenAndUpdated := time.Now().Add(-25 * time.Hour)
	cache := newCacheWithPodInfo(server.URL, lastSeenAndUpdated)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))
	now := time.Now()

	assert.NoError(t, cache.DetectClientFromRequest(req, profile), "detecting pod failed")
	assertPodInfoUpdated(t, cache, server.URL, now)
}

func TestCache_DetectPodFromRequest_whenPodNeverSeenAndCallbackNotReplying_thenReturnErrorAndDoNothing(t *testing.T) {
	cache := NewCache(peerCfg)
	serverURL := fmt.Sprintf("http://localhost:%d", randomPort(t))
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", serverURL))

	err := cache.DetectClientFromRequest(req, profile)
	assert.Error(t, err, "detecting pod should have failed")
	assert.Contains(t, err.Error(), serverURL+"/info", "error message should contain callback URL")
	assertPodInfoNotInserted(t, cache)
}

func TestCache_DetectPodFromRequest_whenPodAlreadySeenAndCallbackNotReplying_thenReturnErrorAndUpdateOnlyLastSeen(t *testing.T) {
	serverURL := fmt.Sprintf("http://localhost:%d", randomPort(t))
	lastSeenAndUpdated := time.Now().Add(-25 * time.Hour)
	cache := newCacheWithPodInfo(serverURL, lastSeenAndUpdated)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", serverURL))
	now := time.Now()

	err := cache.DetectClientFromRequest(req, profile)
	assert.Error(t, err, "detecting pod should have failed")
	assert.Contains(t, err.Error(), serverURL+"/info", "error message should contain callback URL")
	assertPodInfoNotUpdatedExceptLastSeen(t, cache, serverURL, now, lastSeenAndUpdated)
}

func TestCache_DetectPodFromRequest_whenPodNeverSeenAndCallbackReplyingWithHTTPNon200_thenReturnErrorAndDoNothing(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithResponse(t, func(w http.ResponseWriter) {
		w.WriteHeader(404)
		w.Write([]byte("I'm a too old yarnd version which does not support the new /info endpoint"))
	})
	defer cleanup()

	cache := NewCache(peerCfg)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))

	err := cache.DetectClientFromRequest(req, profile)
	assert.EqualError(t, err, fmt.Sprintf("non-success HTTP 404 Not Found response for %s/info", server.URL),
		"detecting pod should have failed")
	assertPodInfoNotInserted(t, cache)
}

func TestCache_DetectPodFromRequest_whenPodAlreadySeenAndCallbackReplyingWithHTTPNon200_thenReturnErrorAndUpdateOnlyLastSeen(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithResponse(t, func(w http.ResponseWriter) {
		w.WriteHeader(404)
		w.Write([]byte("I'm a too old yarnd version which does not support the new /info endpoint"))
	})
	defer cleanup()

	lastSeenAndUpdated := time.Now().Add(-25 * time.Hour)
	cache := newCacheWithPodInfo(server.URL, lastSeenAndUpdated)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))
	now := time.Now()

	err := cache.DetectClientFromRequest(req, profile)
	assert.EqualError(t, err, fmt.Sprintf("non-success HTTP 404 Not Found response for %s/info", server.URL),
		"detecting pod should have failed")
	assertPodInfoNotUpdatedExceptLastSeen(t, cache, server.URL, now, lastSeenAndUpdated)
}

func TestCache_DetectPodFromRequest_whenPodNeverSeenAndCallbackReplyingWithInvalidContentType_thenReturnErrorAndDoNothing(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithResponse(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "improper content type")
		w.WriteHeader(200)
		w.Write([]byte("whoops"))
	})
	defer cleanup()

	cache := NewCache(peerCfg)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))

	err := cache.DetectClientFromRequest(req, profile)
	assert.EqualError(t, err, "mime: expected slash after first token", "detecting pod should have failed")
	assertPodInfoNotInserted(t, cache)
}

func TestCache_DetectPodFromRequest_whenPodAlreadySeenAndCallbackReplyingWithInvalidContentType_thenReturnErrorAndUpdateOnlyLastSeen(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithResponse(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "improper content type")
		w.WriteHeader(200)
		w.Write([]byte("whoops"))
	})
	defer cleanup()

	lastSeenAndUpdated := time.Now().Add(-25 * time.Hour)
	cache := newCacheWithPodInfo(server.URL, lastSeenAndUpdated)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))
	now := time.Now()

	err := cache.DetectClientFromRequest(req, profile)
	assert.EqualError(t, err, "mime: expected slash after first token", "detecting pod should have failed")
	assertPodInfoNotUpdatedExceptLastSeen(t, cache, server.URL, now, lastSeenAndUpdated)
}

func TestCache_DetectPodFromRequest_whenPodNeverSeenAndCallbackReplyingWithNonJSONContentType_thenReturnErrorAndDoNothing(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithResponse(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(200)
		w.Write([]byte("that's no JSON as indicated in the Content-Type header"))
	})
	defer cleanup()

	cache := NewCache(peerCfg)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))

	err := cache.DetectClientFromRequest(req, profile)
	assert.EqualError(t, err, fmt.Sprintf("non-JSON response content type 'text/plain; charset=UTF-8' for %s/info", server.URL),
		"detecting pod should have failed")
	assertPodInfoNotInserted(t, cache)
}

func TestCache_DetectPodFromRequest_whenPodAlreadySeenAndCallbackReplyingWithNonJSONContentType_thenReturnErrorAndUpdateOnlyLastSeen(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithResponse(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(200)
		w.Write([]byte("that's no JSON as indicated in the Content-Type header"))
	})
	defer cleanup()

	lastSeenAndUpdated := time.Now().Add(-25 * time.Hour)
	cache := newCacheWithPodInfo(server.URL, lastSeenAndUpdated)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))
	now := time.Now()

	err := cache.DetectClientFromRequest(req, profile)
	assert.EqualError(t, err, fmt.Sprintf("non-JSON response content type 'text/plain; charset=UTF-8' for %s/info", server.URL),
		"detecting pod should have failed")
	assertPodInfoNotUpdatedExceptLastSeen(t, cache, server.URL, now, lastSeenAndUpdated)
}

func TestCache_DetectPodFromRequest_whenPodNeverSeenAndCallbackReplyingWithJSONGarbage_thenReturnErrorAndDoNothing(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithResponse(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("this is no JSON"))
	})
	defer cleanup()

	cache := NewCache(peerCfg)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))

	err := cache.DetectClientFromRequest(req, profile)
	assert.Error(t, err, "detecting pod should have failed")
	assert.Contains(t, err.Error(), "invalid character", "error message should say something about decoding error")
	assertPodInfoNotInserted(t, cache)
}

func TestCache_DetectPodFromRequest_whenPodAlreadySeenAndCallbackReplyingWithJSONGarbage_thenReturnErrorAndUpdateOnlyLastSeen(t *testing.T) {
	server, cleanup := newCallbackExpectedServerWithResponse(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("this is no JSON"))
	})
	defer cleanup()

	lastSeenAndUpdated := time.Now().Add(-25 * time.Hour)
	cache := newCacheWithPodInfo(server.URL, lastSeenAndUpdated)
	profile := newUserProfile("bob")
	req := newRequestWithUA(fmt.Sprintf("yarnd/0.9001.23@7654321 (+%s/user/bar/twtxt.txt; @bar)", server.URL))
	now := time.Now()

	err := cache.DetectClientFromRequest(req, profile)
	assert.Error(t, err, "detecting pod should have failed")
	assert.Contains(t, err.Error(), "invalid character", "error message should say something about decoding error")
	assertPodInfoNotUpdatedExceptLastSeen(t, cache, server.URL, now, lastSeenAndUpdated)
}
