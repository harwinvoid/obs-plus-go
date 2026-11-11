// Package mockobs is a tiny fake of the Huawei Cloud OBS XML API for the
// end-to-end test. It stores objects in memory, ignores signatures, and
// answers the requests the app makes: list buckets, list objects, put and
// get an object, and delete one or many.
package mockobs

import (
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type object struct {
	data     []byte
	modified time.Time
}

// Server is one fake OBS endpoint.
type Server struct {
	mu             sync.Mutex
	buckets        map[string]map[string]*object
	requests       []string
	denyListBucket bool
	ln             net.Listener
	srv            *http.Server
}

func New() *Server {
	return &Server{buckets: map[string]map[string]*object{}}
}

// Start listens on a free loopback port.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	s.ln = ln
	s.srv = &http.Server{Handler: http.HandlerFunc(s.handle)}
	go func() { _ = s.srv.Serve(ln) }()
	return nil
}

func (s *Server) Close() {
	if s.srv != nil {
		_ = s.srv.Close()
	}
}

// Endpoint is what goes into the app's Endpoint field.
func (s *Server) Endpoint() string {
	return "http://" + s.ln.Addr().String()
}

// AddBucket creates an empty bucket.
func (s *Server) AddBucket(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.buckets[name]; !ok {
		s.buckets[name] = map[string]*object{}
	}
}

// Put stores an object.
func (s *Server) Put(bucket, key, data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[bucket]
	if !ok {
		b = map[string]*object{}
		s.buckets[bucket] = b
	}
	b[key] = &object{data: []byte(data), modified: time.Now().UTC()}
}

// Get returns an object's bytes.
func (s *Server) Get(bucket, key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.buckets[bucket][key]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), o.data...), true
}

// Has reports whether a key exists.
func (s *Server) Has(bucket, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.buckets[bucket][key]
	return ok
}

// Keys lists every key of a bucket, sorted.
func (s *Server) Keys(bucket string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for k := range s.buckets[bucket] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SetDenyListBuckets makes GET / answer 403 AccessDenied, as an account
// without obs:bucket:ListAllMyBuckets sees it.
func (s *Server) SetDenyListBuckets(deny bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.denyListBucket = deny
}

// Requests is the log of requests served, in order.
func (s *Server) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

type s3Owner struct {
	ID string `xml:"ID"`
}

type s3Bucket struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
	Location     string `xml:"Location"`
}

type listBucketsResult struct {
	XMLName xml.Name   `xml:"ListAllMyBucketsResult"`
	Owner   s3Owner    `xml:"Owner"`
	Buckets []s3Bucket `xml:"Buckets>Bucket"`
}

type s3Content struct {
	Key          string  `xml:"Key"`
	LastModified string  `xml:"LastModified"`
	ETag         string  `xml:"ETag"`
	Size         int     `xml:"Size"`
	StorageClass string  `xml:"StorageClass"`
	Owner        s3Owner `xml:"Owner"`
}

type s3CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

type listObjectsResult struct {
	XMLName        xml.Name         `xml:"ListBucketResult"`
	Name           string           `xml:"Name"`
	Prefix         string           `xml:"Prefix"`
	Marker         string           `xml:"Marker"`
	MaxKeys        int              `xml:"MaxKeys"`
	Delimiter      string           `xml:"Delimiter"`
	IsTruncated    bool             `xml:"IsTruncated"`
	NextMarker     string           `xml:"NextMarker,omitempty"`
	Contents       []s3Content      `xml:"Contents"`
	CommonPrefixes []s3CommonPrefix `xml:"CommonPrefixes"`
}

type apiError struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

type deletedObject struct {
	Key string `xml:"Key"`
}

type deleteRequest struct {
	XMLName xml.Name        `xml:"Delete"`
	Objects []deletedObject `xml:"Object"`
}

type deleteResult struct {
	XMLName xml.Name        `xml:"DeleteResult"`
	Deleted []deletedObject `xml:"Deleted"`
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	path := strings.TrimPrefix(r.URL.EscapedPath(), "/")
	rawBucket, rawKey, _ := strings.Cut(path, "/")
	bucket, _ := url.PathUnescape(rawBucket)
	key, _ := url.PathUnescape(rawKey)

	s.mu.Lock()
	s.requests = append(s.requests, fmt.Sprintf("%s %s", r.Method, r.URL.RequestURI()))
	s.mu.Unlock()

	if bucket == "" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "only GET /")
			return
		}
		s.mu.Lock()
		deny := s.denyListBucket
		s.mu.Unlock()
		if deny {
			writeError(w, http.StatusForbidden, "AccessDenied", "Access Denied")
			return
		}
		s.mu.Lock()
		names := make([]string, 0, len(s.buckets))
		for name := range s.buckets {
			names = append(names, name)
		}
		sort.Strings(names)
		out := listBucketsResult{Owner: s3Owner{ID: "mock-owner"}}
		for _, name := range names {
			out.Buckets = append(out.Buckets, s3Bucket{Name: name, CreationDate: "2026-01-01T00:00:00.000Z", Location: "cn-north-4"})
		}
		s.mu.Unlock()
		writeXML(w, http.StatusOK, out)
		return
	}

	query := r.URL.Query()
	switch {
	case r.Method == http.MethodGet && query.Has("location"):
		writeXML(w, http.StatusOK, struct {
			XMLName xml.Name `xml:"LocationConstraint"`
			Value   string   `xml:",chardata"`
		}{Value: "cn-north-4"})
	case r.Method == http.MethodHead:
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet && key == "" && query.Has("delete"):
		writeError(w, http.StatusBadRequest, "InvalidRequest", "delete needs POST")
	case r.Method == http.MethodPost && key == "" && query.Has("delete"):
		s.deleteMany(w, r, bucket)
	case r.Method == http.MethodGet && key == "":
		s.list(w, r, bucket, query.Get("prefix"), query.Get("delimiter"), query.Get("marker"), atoi(query.Get("max-keys"), 1000))
	case r.Method == http.MethodPut && key == "":
		s.mu.Lock()
		if _, ok := s.buckets[bucket]; !ok {
			s.buckets[bucket] = map[string]*object{}
		}
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "IncompleteBody", err.Error())
			return
		}
		s.mu.Lock()
		if _, ok := s.buckets[bucket]; !ok {
			s.buckets[bucket] = map[string]*object{}
		}
		s.buckets[bucket][key] = &object{data: data, modified: time.Now().UTC()}
		s.mu.Unlock()
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet:
		s.mu.Lock()
		o, ok := s.buckets[bucket][key]
		s.mu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprint(len(o.data)))
		w.Header().Set("Last-Modified", o.modified.Format(http.TimeFormat))
		w.Header().Set("ETag", `"mock-etag"`)
		_, _ = w.Write(o.data)
	case r.Method == http.MethodDelete:
		s.mu.Lock()
		delete(s.buckets[bucket], key)
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", r.Method)
	}
}

func (s *Server) list(w http.ResponseWriter, r *http.Request, bucket, prefix, delimiter, marker string, max int) {
	if max <= 0 {
		max = 1000
	}
	s.mu.Lock()
	objs := make(map[string]*object, len(s.buckets[bucket]))
	for k, v := range s.buckets[bucket] {
		objs[k] = v
	}
	s.mu.Unlock()

	keys := make([]string, 0, len(objs))
	for k := range objs {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	type row struct {
		key      string
		folder   bool
		modified time.Time
		size     int
	}
	var rows []row
	seen := map[string]bool{}
	for _, k := range keys {
		rest := strings.TrimPrefix(k, prefix)
		if delimiter != "" {
			if i := strings.Index(rest, delimiter); i >= 0 {
				cp := prefix + rest[:i+len(delimiter)]
				if !seen[cp] {
					seen[cp] = true
					rows = append(rows, row{key: cp, folder: true})
				}
				continue
			}
		}
		rows = append(rows, row{key: k, modified: objs[k].modified, size: len(objs[k].data)})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].key < rows[j].key })

	start := 0
	if marker != "" {
		for start < len(rows) && rows[start].key <= marker {
			start++
		}
	}
	end := start + max
	if end > len(rows) {
		end = len(rows)
	}
	out := listObjectsResult{
		Name: bucket, Prefix: prefix, Marker: marker, MaxKeys: max,
		Delimiter: delimiter, IsTruncated: end < len(rows),
	}
	for _, rw := range rows[start:end] {
		if rw.folder {
			out.CommonPrefixes = append(out.CommonPrefixes, s3CommonPrefix{Prefix: rw.key})
			continue
		}
		out.Contents = append(out.Contents, s3Content{
			Key: rw.key, LastModified: rw.modified.Format("2006-01-02T15:04:05.000Z"),
			ETag: `"mock-etag"`, Size: rw.size, StorageClass: "STANDARD",
			Owner: s3Owner{ID: "mock-owner"},
		})
	}
	if out.IsTruncated && end > start {
		out.NextMarker = rows[end-1].key
	}
	writeXML(w, http.StatusOK, out)
}

func (s *Server) deleteMany(w http.ResponseWriter, r *http.Request, bucket string) {
	var req deleteRequest
	body, _ := io.ReadAll(r.Body)
	if err := xml.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "MalformedXML", err.Error())
		return
	}
	s.mu.Lock()
	for _, o := range req.Objects {
		delete(s.buckets[bucket], o.Key)
	}
	s.mu.Unlock()
	out := deleteResult{Deleted: req.Objects}
	writeXML(w, http.StatusOK, out)
}

func writeXML(w http.ResponseWriter, status int, v any) {
	data, err := xml.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeXML(w, status, apiError{Code: code, Message: message})
}

func atoi(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
}

var _ = io.Discard
