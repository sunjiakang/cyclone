/*
Copyright 2017 caicloud authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	log "github.com/golang/glog"
	gitlab "github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
	gitlabv4 "gopkg.in/xanzy/go-gitlab.v0"

	"github.com/caicloud/cyclone/pkg/api"
	"github.com/caicloud/cyclone/pkg/scm"
	"github.com/caicloud/cyclone/pkg/scm/provider"
)

const (
	apiPathForGitlabVersion = "%s/api/v4/version"

	// gitLabServer represents the server address for public Gitlab.
	gitLabServer = "https://gitlab.com"

	v3APIVersion = "v3"

	v4APIVersion = "v4"
)

var gitlabServerAPIVersions = make(map[string]string)

func init() {
	if err := scm.RegisterProvider(api.Gitlab, NewGitlab); err != nil {
		log.Errorln(err)
	}
}

// NewGitlab news Gitlab v3 or v4 client according to the API version detected from Gitlab server,
func NewGitlab(scmCfg *api.SCMConfig) (scm.SCMProvider, error) {
	version, err := getAPIVersion(scmCfg)
	if err != nil {
		log.Errorf("Fail to get API version for server %s as %v", scmCfg.Server, err)
		return nil, err
	}
	log.Infof("New Gitlab %s client", version)

	switch version {
	case v3APIVersion:
		client, err := newGitlabV3Client(scmCfg.Server, scmCfg.Username, scmCfg.Token)
		if err != nil {
			log.Error("fail to new Gitlab v3 client as %v", err)
			return nil, err
		}

		return &GitlabV3{scmCfg, client}, nil
	case v4APIVersion:
		v4Client, err := newGitlabV4Client(scmCfg.Server, scmCfg.Username, scmCfg.Token)
		if err != nil {
			log.Error("fail to new Gitlab v4 client as %v", err)
			return nil, err
		}

		return &GitlabV4{scmCfg, v4Client}, nil
	default:
		err = fmt.Errorf("Gitlab API version %s is not supported, only support %s and %s", version, v3APIVersion, v4APIVersion)
		log.Errorln(err)
		return nil, err
	}
}

// newGitlabV4Client news Gitlab v4 client by token. If username is empty, use private-token instead of oauth2.0 token.
func newGitlabV4Client(server, username, token string) (*gitlabv4.Client, error) {
	var client *gitlabv4.Client
	if len(username) == 0 {
		client = gitlabv4.NewClient(nil, token)
	} else {
		client = gitlabv4.NewOAuthClient(nil, token)
	}

	if err := client.SetBaseURL(server + "/api/" + v4APIVersion); err != nil {
		log.Error(err.Error())
		return nil, err
	}

	return client, nil
}

// newGitlabV3Client news Gitlab v3 client by token. If username is empty, use private-token instead of oauth2.0 token.
func newGitlabV3Client(server, username, token string) (*gitlab.Client, error) {
	var client *gitlab.Client

	if len(username) == 0 {
		client = gitlab.NewClient(nil, token)
	} else {
		client = gitlab.NewOAuthClient(nil, token)
	}

	if err := client.SetBaseURL(server + "/api/" + v3APIVersion); err != nil {
		log.Error(err.Error())
		return nil, err
	}

	return client, nil
}

func getAPIVersion(scmCfg *api.SCMConfig) (string, error) {
	// Directly get API version if it has been recorded.
	server := provider.ParseServerURL(scmCfg.Server)
	if v, ok := gitlabServerAPIVersions[server]; ok {
		return v, nil
	}

	// Dynamically detect API version if it has not been recorded, and record it for later use.
	version, err := detectAPIVersion(scmCfg)
	if err != nil {
		return "", err
	}

	gitlabServerAPIVersions[server] = version

	return version, nil
}

// versionResponse represents the response of Gitlab version API.
type versionResponse struct {
	Version   string `json:"version"`
	Reversion string `json:"reversion"`
}

func detectAPIVersion(scmCfg *api.SCMConfig) (string, error) {
	if scmCfg.Token == "" {
		token, err := getOauthToken(scmCfg)
		if err != nil {
			log.Error(err)
			return "", err
		}
		scmCfg.Token = token
	}

	url := fmt.Sprintf(apiPathForGitlabVersion, scmCfg.Server)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Error(err)
		return "", err
	}

	// Set headers.
	req.Header.Set("Content-Type", "application/json")
	if scmCfg.Username == "" {
		// Use private token when username is empty.
		req.Header.Set("PRIVATE-TOKEN", scmCfg.Token)
	} else {
		// Use Oauth token when username is not empty.
		req.Header.Set("Authorization", "Bearer "+scmCfg.Token)
	}

	// Use client with redirect disabled, then status code will be 302
	// if Gitlab server does not support /api/v4/version request.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		gv := &versionResponse{}
		err = json.Unmarshal(body, gv)
		if err != nil {
			log.Error(err)
			return "", err
		}

		log.Infof("Gitlab version is %s, will use %s API", gv.Version, v4APIVersion)
		return v4APIVersion, nil
	case http.StatusNotFound, http.StatusFound:
		return v3APIVersion, nil
	default:
		log.Warningf("Status code of Gitlab API version request is %d, use v3 in default", resp.StatusCode)
		return v3APIVersion, nil
	}
}

func getOauthToken(scm *api.SCMConfig) (string, error) {
	if len(scm.Username) == 0 || len(scm.Password) == 0 {
		return "", fmt.Errorf("GitHub username or password is missing")
	}

	bodyData := struct {
		GrantType string `json:"grant_type"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	}{
		GrantType: "password",
		Username:  scm.Username,
		Password:  scm.Password,
	}

	bodyBytes, err := json.Marshal(bodyData)
	if err != nil {
		return "", fmt.Errorf("fail to new request body for token as %s", err.Error())
	}

	// If use the public Gitlab, must use the HTTPS protocol.
	if strings.Contains(scm.Server, "gitlab.com") && strings.HasPrefix(scm.Server, "http://") {
		log.Infof("Convert SCM server from %s to %s to use HTTPS protocol for public Gitlab", scm.Server, gitLabServer)
		scm.Server = gitLabServer
	}

	tokenURL := fmt.Sprintf("%s%s", scm.Server, "/oauth/token")
	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Errorf("Fail to new the request for token as %s", err.Error())
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("Fail to request for token as %s", err.Error())
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Fail to request for token as %s", err.Error())
		return "", err
	}

	if resp.StatusCode/100 == 2 {
		var token oauth2.Token
		err := json.Unmarshal(body, &token)
		if err != nil {
			return "", err
		}
		return token.AccessToken, nil
	}

	err = fmt.Errorf("Fail to request for token as %s", body)
	return "", err
}

func getLanguages(scm *api.SCMConfig, version, project string) (map[string]float32, error) {
	languages := make(map[string]float32)
	path := fmt.Sprintf("%s/api/%s/projects/%s/languages", strings.TrimSuffix(scm.Server, "/"), version, url.QueryEscape(project))
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return languages, err
	}

	if len(scm.Username) == 0 {
		req.Header.Set("PRIVATE-TOKEN", scm.Token)
	} else {
		req.Header.Set("Authorization", "Bearer "+scm.Token)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("Fail to get project languages as %s", err.Error())
		return languages, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Fail to get project languages as %s", err.Error())
		return languages, err
	}

	if resp.StatusCode/100 == 2 {
		err := json.Unmarshal(body, &languages)
		if err != nil {
			return languages, err
		}
		return languages, nil
	}

	err = fmt.Errorf("Fail to get project languages as %s", body)
	return languages, err
}

func getTopLanguage(languages map[string]float32) string {
	var language string
	var max float32
	for l, value := range languages {
		if value > max {
			max = value
			language = l
		}
	}
	return language
}

func getContents(scm *api.SCMConfig, version, project string) ([]RepoFile, error) {
	var files []RepoFile
	path := fmt.Sprintf("%s/api/%s/projects/%s/repository/tree", strings.TrimSuffix(scm.Server, "/"), version, url.QueryEscape(project))
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return files, err
	}

	if len(scm.Username) == 0 {
		req.Header.Set("PRIVATE-TOKEN", scm.Token)
	} else {
		req.Header.Set("Authorization", "Bearer "+scm.Token)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("Fail to get project contents as %s", err.Error())
		return files, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Fail to get project contents as %s", err.Error())
		return files, err
	}

	if resp.StatusCode/100 == 2 {
		err := json.Unmarshal(body, &files)
		if err != nil {
			return files, err
		}
		return files, nil
	}

	err = fmt.Errorf("Fail to get project contents as %s", body)
	return files, err
}

type RepoFile struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	Path string `json:"path,omitempty"`
}

// transStatus trans api.Status to state and description of gitlab statuses.
func transStatus(recordStatus api.Status) (string, string) {
	// GitLab : pending, running, success, failed, canceled.
	state := "pending"
	description := ""

	switch recordStatus {
	case api.Running:
		state = "running"
		description = "The Cyclone CI build is in progress."
	case api.Success:
		state = "success"
		description = "The Cyclone CI build passed."
	case api.Failed:
		state = "failed"
		description = "The Cyclone CI build failed."
	case api.Aborted:
		state = "canceled"
		description = "The Cyclone CI build failed."
	default:
		log.Errorf("not supported state:%s", recordStatus)
	}

	return state, description
}
