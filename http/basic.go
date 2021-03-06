/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package http

import (
	"encoding/base64"
	"errors"
	"net/http"
	"os"

	"github.com/abbot/go-http-auth"
	"github.com/skydive-project/skydive/config"
)

const (
	basicAuthRealm string = "Skydive Authentication"
)

type BasicAuthenticationBackend struct {
	*auth.BasicAuth
	name string
	role string
}

// Name returns the name of the backend
func (b *BasicAuthenticationBackend) Name() string {
	return b.name
}

// DefaultUserRole returns the default user role
func (b *BasicAuthenticationBackend) DefaultUserRole(user string) string {
	return b.role
}

// SetDefaultUserRole defines the default user role
func (b *BasicAuthenticationBackend) SetDefaultUserRole(role string) {
	b.role = role
}

func (b *BasicAuthenticationBackend) Authenticate(username string, password string) (string, error) {
	request := &http.Request{Header: make(http.Header)}
	creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	request.Header.Set("Authorization", "Basic "+creds)

	username = b.CheckAuth(request)
	if username == "" {
		return "", ErrWrongCredentials
	}

	return creds, nil
}

func (b *BasicAuthenticationBackend) Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := authenticateWithHeaders(b, w, r)
		if err != nil {
			Unauthorized(w, r)
			return
		}

		// add "fake" header to let the basic auth library do the authentication
		r.Header.Set("Authorization", "Basic "+token)

		if username := b.CheckAuth(r); username == "" {
			Unauthorized(w, r)
		} else {
			authCallWrapped(w, r, username, wrapped)
		}
	}
}

func NewBasicAuthenticationBackend(name string, provider auth.SecretProvider, role string) (*BasicAuthenticationBackend, error) {
	return &BasicAuthenticationBackend{
		BasicAuth: auth.NewBasicAuthenticator(basicAuthRealm, provider),
		name:      name,
		role:      role,
	}, nil
}

func NewBasicAuthenticationBackendFromConfig(name string) (*BasicAuthenticationBackend, error) {
	role := config.GetString("auth." + name + ".role")
	if role == "" {
		role = defaultUserRole
	}

	var provider auth.SecretProvider
	if file := config.GetString("auth." + name + ".file"); file != "" {
		if _, err := os.Stat(file); err != nil {
			return nil, err
		}

		provider = auth.HtpasswdFileProvider(file)
	} else if users := config.GetStringMapString("auth." + name + ".users"); users != nil && len(users) > 0 {
		provider = NewHtpasswdMapProvider(users).SecretProvider()
	} else {
		return nil, errors.New("No htpassword provider set, you set either file or inline sections")
	}

	return NewBasicAuthenticationBackend(name, provider, role)
}
