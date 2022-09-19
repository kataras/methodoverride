package methodoverride

import (
	"bytes"
	stdContext "context"
	"io/ioutil"
	"net/http"
	"strings"
)

type options struct {
	getters                      []GetterFunc
	methods                      []string
	saveOriginalMethodContextKey interface{} // if not nil original value will be saved.
}

func (o *options) configure(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

func (o *options) canOverride(method string) bool {
	for _, s := range o.methods {
		if s == method {
			return true
		}
	}

	return false
}

func (o *options) get(w http.ResponseWriter, r *http.Request) string {
	for _, getter := range o.getters {
		if v := getter(w, r); v != "" {
			return strings.ToUpper(v)
		}
	}

	return ""
}

// Option sets options for a fresh method override wrapper.
// See `New` package-level function for more.
type Option func(*options)

// Methods can be used to add methods that can be overridden.
// Defaults to "POST".
func Methods(methods ...string) Option {
	for i, s := range methods {
		methods[i] = strings.ToUpper(s)
	}

	return func(opts *options) {
		opts.methods = append(opts.methods, methods...)
	}
}

// SaveOriginalMethod will save the original method
// on Request.Context().Value(requestContextKey).
//
// Defaults to nil, don't save it.
func SaveOriginalMethod(requestContextKey interface{}) Option {
	return func(opts *options) {
		if requestContextKey == nil {
			opts.saveOriginalMethodContextKey = nil
		}
		opts.saveOriginalMethodContextKey = requestContextKey
	}
}

// GetterFunc is the type signature for declaring custom logic
// to extract the method name which a POST request will be replaced with.
type GetterFunc func(http.ResponseWriter, *http.Request) string

// Getter sets a custom logic to use to extract the method name
// to override the POST method with.
// Defaults to nil.
func Getter(customFunc GetterFunc) Option {
	return func(opts *options) {
		opts.getters = append(opts.getters, customFunc)
	}
}

// Headers that client can send to specify a method
// to override the POST method with.
//
// Defaults to:
// X-HTTP-Method
// X-HTTP-Method-Override
// X-Method-Override
func Headers(headers ...string) Option {
	getter := func(w http.ResponseWriter, r *http.Request) string {
		for _, s := range headers {
			if v := r.Header.Get(s); v != "" {
				w.Header().Add("Vary", s)
				return v
			}
		}

		return ""
	}

	return Getter(getter)
}

const postMaxMemory = 32 << 20

// FormField specifies a form field to use to determinate the method
// to override the POST method with.
//
// Example Field:
// <input type="hidden" name="_method" value="DELETE">
//
// Defaults to: "_method".
func FormField(fieldName string) Option {
	return Getter(func(w http.ResponseWriter, r *http.Request) string {
		if form, has := getForm(r, postMaxMemory, true); has {
			if v := form[fieldName]; len(v) > 0 {
				return v[0]
			}
		}
		return ""
	})
}

// getForm returns the request form (url queries, post or multipart) values.
func getForm(r *http.Request, postMaxMemory int64, resetBody bool) (form map[string][]string, found bool) {
	/*
		net/http/request.go#1219
		for k, v := range f.Value {
			r.Form[k] = append(r.Form[k], v...)
			// r.PostForm should also be populated. See Issue 9305.
			r.PostForm[k] = append(r.PostForm[k], v...)
		}
	*/

	if form := r.Form; len(form) > 0 {
		return form, true
	}

	if form := r.PostForm; len(form) > 0 {
		return form, true
	}

	if m := r.MultipartForm; m != nil {
		if len(m.Value) > 0 {
			return m.Value, true
		}
	}

	var bodyCopy []byte

	if resetBody {
		// on POST, PUT and PATCH it will read the form values from request body otherwise from URL queries.
		if m := r.Method; m == "POST" || m == "PUT" || m == "PATCH" {
			bodyCopy, _ = getBody(r, resetBody)
			if len(bodyCopy) == 0 {
				return nil, false
			}
			// r.Body = ioutil.NopCloser(io.TeeReader(r.Body, buf))
		} else {
			resetBody = false
		}
	}

	// ParseMultipartForm calls `request.ParseForm` automatically
	// therefore we don't need to call it here, although it doesn't hurt.
	// After one call to ParseMultipartForm or ParseForm,
	// subsequent calls have no effect, are idempotent.
	err := r.ParseMultipartForm(postMaxMemory)
	if resetBody {
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyCopy))
	}
	if err != nil && err != http.ErrNotMultipart {
		return nil, false
	}

	if form := r.Form; len(form) > 0 {
		return form, true
	}

	if form := r.PostForm; len(form) > 0 {
		return form, true
	}

	if m := r.MultipartForm; m != nil {
		if len(m.Value) > 0 {
			return m.Value, true
		}
	}

	return nil, false
}

// getBody reads and returns the request body.
func getBody(r *http.Request, resetBody bool) ([]byte, error) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	if resetBody {
		// * remember, Request.Body has no Bytes(), we have to consume them first
		// and after re-set them to the body, this is the only solution.
		r.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	}

	return data, nil
}

// Query specifies a url parameter name to use to determinate the method
// to override the POST methos with.
//
// Example URL Query string:
// http://localhost:8080/path?_method=DELETE
//
// Defaults to: "_method".
func Query(paramName string) Option {
	getter := func(w http.ResponseWriter, r *http.Request) string {
		return r.URL.Query().Get(paramName)
	}

	return Getter(getter)
}

// Only clears all default or previously registered values
// and uses only the "o" option(s).
//
// The default behavior is to check for all the following by order:
// headers, form field, query string
// and any custom getter (if set).
// Use this method to override that
// behavior and use only the passed option(s)
// to determinate the method to override with.
//
// Use cases:
//
//  1. When need to check only for headers and ignore other fields:
//     New(Only(Headers("X-Custom-Header")))
//
//  2. When need to check only for (first) form field and (second) custom getter:
//     New(Only(FormField("fieldName"), Getter(...)))
func Only(o ...Option) Option {
	return func(opts *options) {
		opts.getters = opts.getters[0:0]
		opts.configure(o...)
	}
}

// New returns a new method override wrapper
// which can be registered on any HTTP server.
//
// Use this wrapper when you expecting clients
// that do not support certain HTTP operations such as DELETE or PUT for security reasons.
// This wrapper will accept a method, based on criteria, to override the POST method with.
func New(opt ...Option) func(next http.Handler) http.Handler {
	opts := new(options)
	// Default values.
	opts.configure(
		Methods(http.MethodPost),
		Headers("X-HTTP-Method", "X-HTTP-Method-Override", "X-Method-Override"),
		FormField("_method"),
		Query("_method"),
	)
	opts.configure(opt...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			originalMethod := strings.ToUpper(r.Method)
			if opts.canOverride(originalMethod) {
				newMethod := opts.get(w, r)
				if newMethod != "" {
					if opts.saveOriginalMethodContextKey != nil {
						r = r.WithContext(stdContext.WithValue(r.Context(), opts.saveOriginalMethodContextKey, originalMethod))
					}
					r.Method = newMethod
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
