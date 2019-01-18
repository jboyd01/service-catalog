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
	goflag "flag"
	"os"
	"time"

	etcdapi "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	"github.com/coreos/etcd-operator/pkg/client"
	"github.com/coreos/etcd-operator/pkg/generated/clientset/versioned"
	"github.com/coreos/etcd-operator/pkg/util/retryutil"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	pflag "github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var options *Context

// Execute starts the HTTP Server and runs the health check tasks on a periodic basis
func Execute() error {
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	options = NewContext()
	options.AddFlags(pflag.CommandLine)
	pflag.CommandLine.Set("alsologtostderr", "true")
	defer glog.Flush()
	return rootCmd.Execute()

}

var rootCmd = &cobra.Command{
	Use:   "init-etcd",
	Short: "init-etcd creates EtcdCluster resource for Service Catalog",
	Long: "init-etcd created EtcdCluster resource which is used by etcd operator " +
		"to create etcd instance for Service Catalog",
	Run: func(cmd *cobra.Command, args []string) {
		h, err := NewInitEtcd(options)
		if err != nil {
			glog.Errorf("Error initializing: %v", err)
			os.Exit(1)
		}

		err = h.RunInitEtcd(options)
		if err != nil {
			glog.Errorf("Error: %v", err)
			os.Exit(1)
		}
	},
}

// InitEtcd is a type that used to control various aspects of the health
// check.
type InitEtcd struct {
	kubeClientSet  kubernetes.Interface
	etcdClient     versioned.Interface
	frameworkError error
}

type acceptFunc func(*etcdapi.EtcdCluster) bool

var retryInterval = 10 * time.Second

// NewInitEtcd creates a new InitEtcd object and initializes the kube
// client
func NewInitEtcd(s *Context) (*InitEtcd, error) {
	h := &InitEtcd{}
	var kubeConfig *rest.Config

	// If token exists assume we are running in a pod
	_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err == nil {
		kubeConfig, err = rest.InClusterConfig()
	} else {
		kubeConfig, err = LoadConfig(s.KubeConfig, s.KubeContext)
	}

	if err != nil {
		return nil, err
	}

	h.kubeClientSet, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		glog.Errorf("Error creating kubeClientSet: %v", err)
		return nil, err
	}

	h.etcdClient = client.MustNew(kubeConfig)
	return h, nil
}

func createCluster(crClient versioned.Interface, namespace string, cl *etcdapi.EtcdCluster) (*etcdapi.EtcdCluster, error) {
	cl.Namespace = namespace
	res, err := crClient.EtcdV1beta2().EtcdClusters(namespace).Create(cl)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func newCluster(genName string, size int) *etcdapi.EtcdCluster {
	return &etcdapi.EtcdCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       etcdapi.EtcdClusterResourceKind,
			APIVersion: etcdapi.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: genName,
			Annotations: map[string]string{
				"etcd.database.coreos.com/scope": "clusterwide",
			},
		},
		Spec: etcdapi.ClusterSpec{
			Size: size,
		},
	}
}

func waitUntilSizeReached(crClient versioned.Interface, size, retries int, cl *etcdapi.EtcdCluster, accepts ...acceptFunc) ([]string, error) {
	var names []string
	err := retryutil.Retry(retryInterval, retries, func() (done bool, err error) {
		currCluster, err := crClient.EtcdV1beta2().EtcdClusters(cl.Namespace).Get(cl.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, accept := range accepts {
			if !accept(currCluster) {
				return false, nil
			}
		}

		names = currCluster.Status.Members.Ready
		glog.Infof("waiting for %d etcd pods, healthy etcd members: %v", size, names)
		if len(names) != size {
			return false, nil
		}
		return true, nil
	})

	return names, err
}

// RunInitEtcd
func (h *InitEtcd) RunInitEtcd(s *Context) error {

	// only create EtcdCluster resource if it doesn't exist
	myEtcd, err := h.etcdClient.EtcdV1beta2().EtcdClusters("kube-service-catalog").Get("svccat-etcd", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) {
		cl := newCluster("svccat-etcd", 3)
		glog.Infof("Creating %+v in %s", cl, "kube-service-catalog")
		myEtcd, err = createCluster(h.etcdClient, "kube-service-catalog", cl)
		if err != nil {
			return (err)
		}
	}
	names, err := waitUntilSizeReached(h.etcdClient, 3, 18, myEtcd)
	if err == nil {
		glog.Infof("Etcd cluster is ready: %+v", names)
	}
	return err
}
