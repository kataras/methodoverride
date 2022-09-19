package methodoverride

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMethodOverride(t *testing.T) {
	mo := New(
		// Defaults to nil.
		//
		SaveOriginalMethod("_originalMethod"),
		// Default values.
		//
		// Methods(http.MethodPost),
		// Headers("X-HTTP-Method", "X-HTTP-Method-Override", "X-Method-Override"),
		// FormField("_method"),
		// Query("_method"),
	)

	router := http.NewServeMux()

	var (
		expectedDelResponse  = "delete resp"
		expectedPostResponse = "post resp"
	)

	router.HandleFunc("/path", func(w http.ResponseWriter, r *http.Request) {
		resp := expectedPostResponse
		if r.Method == http.MethodDelete {
			resp = expectedDelResponse
		}

		w.Write([]byte(resp))
	})

	router.HandleFunc("/path2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		fmt.Fprintf(w, "%s%s", expectedDelResponse, r.Context().Value("_originalMethod"))
	})

	// Wrap it to give a methodoverride-enabled router.
	// http.ListenAndServe(":8080", mo(router))
	srv := httptest.NewServer(mo(router))
	defer srv.Close()

	// Test headers.
	expect(t, http.MethodPost, srv.URL+"/path", withHeader("X-HTTP-Method", http.MethodDelete)).
		statusCode(http.StatusOK).bodyEq(expectedDelResponse)
	expect(t, http.MethodPost, srv.URL+"/path", withHeader("X-HTTP-Method-Override", http.MethodDelete)).
		statusCode(http.StatusOK).bodyEq(expectedDelResponse)
	expect(t, http.MethodPost, srv.URL+"/path", withHeader("X-Method-Override", http.MethodDelete)).
		statusCode(http.StatusOK).bodyEq(expectedDelResponse)

	// Test form field value.
	expect(t, http.MethodPost, srv.URL+"/path", withFormField("_method", http.MethodDelete)).
		statusCode(http.StatusOK).bodyEq(expectedDelResponse)

	// Test URL Query (although it's the same as form field in this case).
	expect(t, http.MethodPost, srv.URL+"/path", withQuery("_method", http.MethodDelete)).
		statusCode(http.StatusOK).bodyEq(expectedDelResponse)

	// Test saved original method and test without registered "POST" route.
	expect(t, http.MethodPost, srv.URL+"/path2", withQuery("_method", http.MethodDelete)).
		statusCode(http.StatusOK).bodyEq(expectedDelResponse + http.MethodPost)

	// Test simple POST request without method override fields.
	expect(t, http.MethodPost, srv.URL+"/path").
		statusCode(http.StatusOK).bodyEq(expectedPostResponse)

	// Test simple DELETE request.
	expect(t, http.MethodDelete, srv.URL+"/path").
		statusCode(http.StatusOK).bodyEq(expectedDelResponse)
}

// Small test suite for this package follows.

func expect(t *testing.T, method, url string, testieOptions ...func(*http.Request)) *testie {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, opt := range testieOptions {
		opt(req)
	}

	return testReq(t, req)
}

func withHeader(key string, value string) func(*http.Request) {
	return func(r *http.Request) {
		r.Header.Add(key, value)
	}
}

func withQuery(key string, value string) func(*http.Request) {
	return func(r *http.Request) {
		q := r.URL.Query()
		q.Add(key, value)

		enc := strings.NewReader(q.Encode())
		r.Body = ioutil.NopCloser(enc)
		r.GetBody = func() (io.ReadCloser, error) { return http.NoBody, nil }

		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
}

func withFormField(key string, value string) func(*http.Request) {
	return func(r *http.Request) {
		if r.Form == nil {
			r.Form = make(url.Values)
		}
		r.Form.Add(key, value)

		enc := strings.NewReader(r.Form.Encode())
		r.Body = ioutil.NopCloser(enc)
		r.ContentLength = int64(enc.Len())

		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
}

func testReq(t *testing.T, req *http.Request) *testie {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	resp.Request = req
	return &testie{t: t, resp: resp}
}

type testie struct {
	t    *testing.T
	resp *http.Response
}

func (te *testie) statusCode(expected int) *testie {
	if got := te.resp.StatusCode; expected != got {
		te.t.Fatalf("%s: expected status code: %d but got %d", te.resp.Request.URL, expected, got)
	}

	return te
}

func (te *testie) bodyEq(expected string) *testie {
	b, err := ioutil.ReadAll(te.resp.Body)
	te.resp.Body.Close()
	if err != nil {
		te.t.Fatal(err)
	}

	if got := string(b); expected != got {
		te.t.Fatalf("%s: expected to receive '%s' but got '%s'", te.resp.Request.URL, expected, got)
	}

	return te
}
