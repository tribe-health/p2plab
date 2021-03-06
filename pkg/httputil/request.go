// Copyright 2019 Netflix, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package httputil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Netflix/p2plab/errdefs"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Request struct {
	Method  string
	Url     string
	Options map[string]string
	body    io.Reader

	client    *retryablehttp.Client
	rawClient *http.Client
}

func (r *Request) Option(key string, value interface{}) *Request {
	var s string
	switch v := value.(type) {
	case bool:
		s = strconv.FormatBool(v)
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		s = fmt.Sprint(value)
	}

	r.Options[key] = s
	return r
}

func (r *Request) Body(value interface{}) *Request {
	var reader io.Reader
	switch v := value.(type) {
	case []byte:
		reader = bytes.NewReader(v)
	case string:
		reader = bytes.NewReader([]byte(v))
	case io.Reader:
		reader = v
	}

	r.body = reader
	return r
}

func (r *Request) Send(ctx context.Context) (*http.Response, error) {
	u := r.url()
	req, err := http.NewRequest(r.Method, u, r.body)
	if err != nil {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "failed to create new http request")
	}
	req = req.WithContext(ctx)

	span := opentracing.SpanFromContext(ctx)
	if span != nil {
		var ht *nethttp.Tracer
		req, ht = nethttp.TraceRequest(span.Tracer(), req)
		defer ht.Finish()
	}

	retryablereq, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Do(retryablereq)
	if err != nil {
		return resp, errors.Wrap(err, "failed to do http request")
	}

	if (resp.StatusCode >= 400 && resp.StatusCode <= 499) ||
		resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error().Msgf("failed to read body: %s", err)
		}
		defer resp.Body.Close()

		return nil, errors.Errorf("server rejected request [%d]: %s", resp.StatusCode, body)
	}

	return resp, nil
}

func (r *Request) url() string {
	values := make(url.Values)
	for k, v := range r.Options {
		values.Add(k, v)
	}
	return fmt.Sprintf("%s?%s", r.Url, values.Encode())
}
