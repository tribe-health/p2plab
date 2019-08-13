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

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Request struct {
	Client   *http.Client
	Method   string
	Base     string
	Endpoint string
	Options  map[string]string

	body io.Reader
}

func NewRequest(client *http.Client, base, method, path string, a ...interface{}) *Request {
	return &Request{
		Client:   client,
		Method:   method,
		Base:     base,
		Endpoint: fmt.Sprintf(path, a...),
		Options:  make(map[string]string),
	}
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
	req, err := http.NewRequest(r.Method, r.url(), r.body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new http request")
	}

	// dump, err := httputil.DumpRequest(req, false)
	// if err != nil {
	// 	return nil, err
	// }
	// fmt.Printf("dump:\n%s\n", string(dump))

	req = req.WithContext(ctx)
	resp, err := r.Client.Do(req)
	if err != nil {
		return resp, errors.Wrap(err, "failed to do http request")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error().Msgf("failed to read body: %s", err)
		}

		return nil, errors.Errorf("got bad status [%d]: %s", resp.StatusCode, body)
	}

	return resp, nil
}

func (r *Request) url() string {
	values := make(url.Values)
	for k, v := range r.Options {
		values.Add(k, v)
	}

	return fmt.Sprintf("%s%s?%s", r.Base, r.Endpoint, values.Encode())
}
