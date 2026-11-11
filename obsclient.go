package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

// entry is one row of the object list: a folder or an object.
type entry struct {
	Key      string // full key, folders end in "/"
	Name     string // name shown in the list
	Folder   bool
	Size     int64
	Class    string
	Modified string
}

// obsClient wraps the Huawei Cloud OBS SDK for the few operations this app
// needs.
type obsClient struct {
	c        *obs.ObsClient
	endpoint string
}

func normalizeEndpoint(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("请填写 Endpoint")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	s = strings.TrimSuffix(s, "/")
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("Endpoint 无效：%s", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("Endpoint 只支持 http 或 https：%s", raw)
	}
	return s, nil
}

func dialOBS(ak, sk, endpoint string) (*obsClient, error) {
	ep, err := normalizeEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	c, err := obs.New(ak, sk, ep, obs.WithMaxRetryCount(2))
	if err != nil {
		return nil, fmt.Errorf("创建 OBS 客户端失败：%w", err)
	}
	return &obsClient{c: c, endpoint: ep}, nil
}

func (o *obsClient) close() {
	if o != nil && o.c != nil {
		o.c.Close()
	}
}

// errText turns the SDK's error into a line worth showing the user.
func errText(err error) string {
	var oe obs.ObsError
	if errors.As(err, &oe) {
		return formatObsError(oe)
	}
	var oep *obs.ObsError
	if errors.As(err, &oep) && oep != nil {
		return formatObsError(*oep)
	}
	return err.Error()
}

func formatObsError(e obs.ObsError) string {
	requestID := ""
	if e.RequestId != "" {
		requestID = "（RequestId " + e.RequestId + "）"
	}
	switch e.Code {
	case "AccessDenied", "SignatureDoesNotMatch", "InvalidAccessKeyId":
		return "访问被拒绝：AK/SK 无此操作权限，或 Endpoint 与桶所在区域不一致" + requestID
	case "NoSuchBucket":
		return "存储桶不存在，请检查名称与区域" + requestID
	case "NoSuchKey":
		return "对象不存在" + requestID
	case "InvalidBucketName":
		return "存储桶名称无效" + requestID
	}
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = strings.TrimSpace(e.Code)
	}
	if msg == "" {
		msg = e.Status
	}
	if e.Code != "" && !strings.Contains(msg, e.Code) {
		msg = e.Code + "：" + msg
	}
	return msg + requestID
}

func (o *obsClient) buckets() ([]string, error) {
	out, err := o.c.ListBuckets(&obs.ListBucketsInput{})
	if err != nil {
		return nil, errors.New(errText(err))
	}
	names := make([]string, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		names = append(names, b.Name)
	}
	sort.Strings(names)
	return names, nil
}

// list returns the folders and objects directly under prefix. truncated
// reports that the bucket has more rows than one request returns.
func (o *obsClient) list(bucket, prefix string) ([]entry, bool, error) {
	out, err := o.c.ListObjects(&obs.ListObjectsInput{
		ListObjsInput: obs.ListObjsInput{Prefix: prefix, Delimiter: "/", MaxKeys: 1000},
		Bucket:        bucket,
	})
	if err != nil {
		return nil, false, errors.New(errText(err))
	}
	rows := make([]entry, 0, len(out.CommonPrefixes)+len(out.Contents))
	for _, p := range out.CommonPrefixes {
		name := strings.TrimSuffix(strings.TrimPrefix(p, prefix), "/")
		if name == "" {
			continue
		}
		rows = append(rows, entry{Key: p, Name: name, Folder: true})
	}
	for _, c := range out.Contents {
		name := strings.TrimPrefix(c.Key, prefix)
		// A key under a subfolder, or the folder marker itself.
		if name == "" || strings.Contains(name, "/") {
			continue
		}
		rows = append(rows, entry{
			Key:      c.Key,
			Name:     name,
			Size:     c.Size,
			Class:    string(c.StorageClass),
			Modified: c.LastModified.Local().Format("2006-01-02 15:04"),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Folder != rows[j].Folder {
			return rows[i].Folder
		}
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	return rows, out.IsTruncated, nil
}

type progressFunc func(done, total int64)

func (p progressFunc) ProgressChanged(e *obs.ProgressEvent) {
	if p != nil {
		p(e.ConsumedBytes, e.TotalBytes)
	}
}

// upload sends one file to bucket/key. ponytail: one PutFile per file, so a
// single object tops out at OBS's 5 GB PUT limit and does not resume; switch
// to UploadFile (multipart, resumable) when large files matter.
func (o *obsClient) upload(bucket, key, path string, progress progressFunc) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return errors.New("不能上传文件夹")
	}
	_, err = o.c.PutFile(&obs.PutFileInput{
		PutObjectBasicInput: obs.PutObjectBasicInput{
			ObjectOperationInput: obs.ObjectOperationInput{Bucket: bucket, Key: key},
			ContentLength:        st.Size(),
		},
		SourceFile: path,
	}, obs.WithProgress(progress))
	if err != nil {
		return errors.New(errText(err))
	}
	return nil
}

// download writes bucket/key to dest, replacing it only once the whole
// object has arrived.
func (o *obsClient) download(bucket, key, dest string, progress progressFunc) error {
	out, err := o.c.GetObject(&obs.GetObjectInput{
		GetObjectMetadataInput: obs.GetObjectMetadataInput{Bucket: bucket, Key: key},
	})
	if err != nil {
		return errors.New(errText(err))
	}
	defer out.Body.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".obsgo-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	buf := make([]byte, 64*1024)
	var done int64
	for {
		n, rerr := out.Body.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				tmp.Close()
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, out.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			tmp.Close()
			return rerr
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return err
	}
	return nil
}

// read fetches an object for a preview, refusing ones over limit bytes.
func (o *obsClient) read(bucket, key string, limit int64) ([]byte, error) {
	out, err := o.c.GetObject(&obs.GetObjectInput{
		GetObjectMetadataInput: obs.GetObjectMetadataInput{Bucket: bucket, Key: key},
	})
	if err != nil {
		return nil, errors.New(errText(err))
	}
	defer out.Body.Close()
	if out.ContentLength > limit {
		return nil, fmt.Errorf("%s 超过预览上限（%s）", humanSize(out.ContentLength), humanSize(limit))
	}
	data, err := io.ReadAll(io.LimitReader(out.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("对象超过预览上限（%s）", humanSize(limit))
	}
	return data, nil
}

// signedURL makes a temporary GET link, as "create object link" does in OBS
// Browser+.
func (o *obsClient) signedURL(bucket, key string, expires int) (string, error) {
	out, err := o.c.CreateSignedUrl(&obs.CreateSignedUrlInput{
		Method: obs.HttpMethodGet, Bucket: bucket, Key: key, Expires: expires,
	})
	if err != nil {
		return "", errors.New(errText(err))
	}
	return out.SignedUrl, nil
}

// objectProps is what the properties dialog shows: metadata from HEAD and
// the object's ACL.
type objectProps struct {
	Size        int64
	Class       string
	ContentType string
	ETag        string
	Modified    time.Time
	Metadata    map[string]string
	Owner       string
	Grants      []string
}

func (o *obsClient) props(bucket, key string) (objectProps, error) {
	var p objectProps
	out, err := o.c.GetObjectMetadata(&obs.GetObjectMetadataInput{Bucket: bucket, Key: key})
	if err != nil {
		return p, errors.New(errText(err))
	}
	p = objectProps{
		Size:        out.ContentLength,
		Class:       string(out.StorageClass),
		ContentType: out.ContentType,
		ETag:        strings.Trim(out.ETag, `"`),
		Modified:    out.LastModified.Local(),
		Metadata:    out.Metadata,
	}
	if acl, aerr := o.c.GetObjectAcl(&obs.GetObjectAclInput{Bucket: bucket, Key: key}); aerr == nil {
		p.Owner = acl.Owner.ID
		for _, g := range acl.Grants {
			who := g.Grantee.ID
			if who == "" {
				who = string(g.Grantee.URI)
			}
			if who == "" {
				who = g.Grantee.DisplayName
			}
			p.Grants = append(p.Grants, who+" · "+string(g.Permission))
		}
	}
	return p, nil
}

// copyObject copies one object within a bucket.
func (o *obsClient) copyObject(bucket, src, dst string) error {
	_, err := o.c.CopyObject(&obs.CopyObjectInput{
		ObjectOperationInput: obs.ObjectOperationInput{Bucket: bucket, Key: dst},
		CopySourceBucket:     bucket,
		CopySourceKey:        src,
	})
	if err != nil {
		return errors.New(errText(err))
	}
	return nil
}

// listAll lists every object under prefix, without folders: what a folder
// download needs.
func (o *obsClient) listAll(bucket, prefix string) ([]entry, error) {
	var out []entry
	marker := ""
	for {
		res, err := o.c.ListObjects(&obs.ListObjectsInput{
			ListObjsInput: obs.ListObjsInput{Prefix: prefix, MaxKeys: 1000},
			Bucket:        bucket,
			Marker:        marker,
		})
		if err != nil {
			return nil, errors.New(errText(err))
		}
		for _, c := range res.Contents {
			if strings.HasSuffix(c.Key, "/") && c.Size == 0 {
				continue
			}
			out = append(out, entry{Key: c.Key, Name: path.Base(c.Key), Size: c.Size})
		}
		if !res.IsTruncated {
			return out, nil
		}
		if res.NextMarker != "" {
			marker = res.NextMarker
		} else if len(res.Contents) > 0 {
			marker = res.Contents[len(res.Contents)-1].Key
		} else {
			return out, nil
		}
	}
}

func (o *obsClient) createBucket(name, region string) error {
	in := &obs.CreateBucketInput{Bucket: name}
	if region != "" {
		in.BucketLocation = obs.BucketLocation{Location: region}
	}
	if _, err := o.c.CreateBucket(in); err != nil {
		return errors.New(errText(err))
	}
	return nil
}

func (o *obsClient) deleteBucket(name string) error {
	if _, err := o.c.DeleteBucket(name); err != nil {
		return errors.New(errText(err))
	}
	return nil
}

func (o *obsClient) newFolder(bucket, key string) error {
	_, err := o.c.NewFolder(&obs.NewFolderInput{
		ObjectOperationInput: obs.ObjectOperationInput{Bucket: bucket, Key: key},
	})
	if err != nil {
		return errors.New(errText(err))
	}
	return nil
}

func (o *obsClient) remove(bucket, key string) error {
	_, err := o.c.DeleteObject(&obs.DeleteObjectInput{Bucket: bucket, Key: key})
	if err != nil {
		return errors.New(errText(err))
	}
	return nil
}

// removeFolder deletes the folder marker and everything under prefix: OBS
// folders exist only as key prefixes.
func (o *obsClient) removeFolder(bucket, prefix string) error {
	marker := ""
	for {
		out, err := o.c.ListObjects(&obs.ListObjectsInput{
			ListObjsInput: obs.ListObjsInput{Prefix: prefix, MaxKeys: 1000},
			Bucket:        bucket,
			Marker:        marker,
		})
		if err != nil {
			return errors.New(errText(err))
		}
		keys := make([]obs.ObjectToDelete, 0, len(out.Contents))
		for _, obj := range out.Contents {
			keys = append(keys, obs.ObjectToDelete{Key: obj.Key})
		}
		if len(keys) > 0 {
			if _, err := o.c.DeleteObjects(&obs.DeleteObjectsInput{Bucket: bucket, Objects: keys}); err != nil {
				return errors.New(errText(err))
			}
		}
		if !out.IsTruncated {
			return nil
		}
		if out.NextMarker != "" {
			marker = out.NextMarker
		} else if len(out.Contents) > 0 {
			marker = out.Contents[len(out.Contents)-1].Key
		} else {
			return nil
		}
	}
}

// configuredEndpoint is where the app signs in: Huawei Cloud's global OBS
// endpoint, which lists buckets and reports each bucket's region. A private
// deployment sets OBSGO_ENDPOINT, and then buckets are reached through it
// directly.
func configuredEndpoint() string {
	if ep := strings.TrimSpace(os.Getenv("OBSGO_ENDPOINT")); ep != "" {
		return ep
	}
	return "https://obs.myhuaweicloud.com"
}

// usesRegionalEndpoints reports whether bucket operations must target the
// bucket's own region, as they do on Huawei Cloud's global endpoint.
func usesRegionalEndpoints(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && u.Host == "obs.myhuaweicloud.com"
}

func regionalEndpoint(region string) string {
	return "https://obs." + region + ".myhuaweicloud.com"
}

// location is the region a bucket lives in.
func (o *obsClient) location(bucket string) (string, error) {
	out, err := o.c.GetBucketLocation(bucket)
	if err != nil {
		return "", errors.New(errText(err))
	}
	return strings.TrimSpace(out.Location), nil
}

// verifyBucketAccess checks that the keys may open a bucket at prefix. The
// global endpoint answers bucket listings but not always object requests, so
// a failure asks where the bucket is and tries that region.
func verifyBucketAccess(global *obsClient, ak, sk, bucket, prefix string) error {
	_, _, err := global.list(bucket, prefix)
	if err == nil {
		return nil
	}
	if loc, lerr := global.location(bucket); lerr == nil && loc != "" {
		if regional, derr := dialOBS(ak, sk, regionalEndpoint(loc)); derr == nil {
			defer regional.close()
			if _, _, rerr := regional.list(bucket, prefix); rerr == nil {
				return nil
			}
		}
	}
	return err
}
