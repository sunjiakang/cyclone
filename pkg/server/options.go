/*
Copyright 2016 caicloud authors. All rights reserved.

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

package server

import (
	"time"

	cli "gopkg.in/urfave/cli.v1"
)

// api server env
const (
	// MongoDBHost ...
	MongoDBHost = "MONGODB_HOST"
	// SaltKey ...
	SaltKey = "SALT_KEY"
	// CloudAutoDiscovery ...
	CloudAutoDiscovery = "CLOUD_AUTO_DISCOVERY"
	// RecordLogRotationThreshold ...
	RecordRotationThreshold = "RECORD_ROTATION_THRESHOLD"
	// NotificationURL ...
	NotificationURL = "NOTIFICATION_URL"
	// RecordWebURLTemplate is a customer's pipeline record website URL address template.
	RecordWebURLTemplate = "RECORD_WEB_URL_TEMPLATE"
)

// APIServerOptions contains all options(config) for api server
type APIServerOptions struct {
	MongoDBHost             string
	SaltKey                 string
	MongoGracePeriod        time.Duration
	CyclonePort             int
	CycloneAddrTemplate     string
	ShowAPIDoc              bool
	CloudAutoDiscovery      bool
	RecordRotationThreshold int
	NotificationURL         string
	RecordWebURLTemplate    string
}

// NewAPIServerOptions returns a new APIServerOptions
func NewAPIServerOptions() *APIServerOptions {
	return &APIServerOptions{
		MongoGracePeriod:    30 * time.Second,
		CyclonePort:         7099,
		CycloneAddrTemplate: "http://localhost:%v",
	}
}

// AddFlags adds flags to cli.App
func (opts *APIServerOptions) AddFlags(app *cli.App) {

	flags := []cli.Flag{
		cli.StringFlag{
			Name:        "mongodb-host",
			Value:       "localhost",
			Usage:       "mongdb host",
			EnvVar:      MongoDBHost,
			Destination: &opts.MongoDBHost,
		},
		cli.StringFlag{
			Name:        "salt-key",
			Value:       "caicloud-cyclone",
			Usage:       "salt key to encrypt passwords",
			EnvVar:      SaltKey,
			Destination: &opts.SaltKey,
		},
		cli.BoolTFlag{
			Name:        "show-api-doc",
			Usage:       "show the api doc at http://<cyclone instance>/apidocs/#/api/v0.1",
			Destination: &opts.ShowAPIDoc,
		},
		cli.BoolTFlag{
			Name:        "cloud-auto-discovery",
			Usage:       "auto discovery cloud by k8s serviceAccount, default to true",
			EnvVar:      CloudAutoDiscovery,
			Destination: &opts.CloudAutoDiscovery,
		},
		cli.IntFlag{
			Name:        "record-rotation-threshold",
			Usage:       "pipeline record rotation threshold",
			EnvVar:      RecordRotationThreshold,
			Destination: &opts.RecordRotationThreshold,
		},
		cli.StringFlag{
			Name:        "notification-url",
			Usage:       "Notification URL",
			EnvVar:      NotificationURL,
			Destination: &opts.NotificationURL,
		},
		cli.StringFlag{
			Name:        "record-web-url-template",
			Usage:       "Record web URL template",
			EnvVar:      RecordWebURLTemplate,
			Destination: &opts.RecordWebURLTemplate,
		},
	}

	app.Flags = append(app.Flags, flags...)
}
