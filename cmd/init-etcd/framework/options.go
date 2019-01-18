/*
Copyright 2018 The Kubernetes Authors.

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

package framework

import (
	"os"

	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/clientcmd"
)

// Context is the main context object for the health check server
type Context struct {
	KubeHost    string
	KubeConfig  string
	KubeContext string
}

// NewContext creates a new Context with a default config.
func NewContext() *Context {
	s := Context{}
	return &s
}

// AddFlags adds flags to the specified FlagSet.
func (s *Context) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.KubeHost, "kubernetes-host", "http://127.0.0.1:8080", "The kubernetes host, or apiserver, to connect to")
	fs.StringVar(&s.KubeConfig, "kubernetes-config", os.Getenv(clientcmd.RecommendedConfigPathEnvVar), "Path to config containing embedded authinfo for kubernetes. Default value is from environment variable "+clientcmd.RecommendedConfigPathEnvVar)
	fs.StringVar(&s.KubeContext, "kubernetes-context", "", "config context to use for kubernetes. If unset, will use value from 'current-context'")
}
