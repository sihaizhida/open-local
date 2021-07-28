/*
Copyright 2021 OECP Authors.

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

package controller

import (
	"os"
	"strconv"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	snapshot "github.com/kubernetes-csi/external-snapshotter/client/v3/clientset/versioned"
	clientset "github.com/oecp/open-local-storage-service/pkg/generated/clientset/versioned"

	lsstype "github.com/oecp/open-local-storage-service/pkg"
	"github.com/oecp/open-local-storage-service/pkg/agent/common"
	"github.com/oecp/open-local-storage-service/pkg/agent/discovery"
)

// NewAgent returns a new open-local-storage-service agent
func NewAgent(
	config *common.Configuration,
	kubeclientset kubernetes.Interface,
	lssclientset clientset.Interface,
	snapclientset snapshot.Interface) *Agent {

	controller := &Agent{
		Configuration: config,
		kubeclientset: kubeclientset,
		lssclientset:  lssclientset,
		snapclientset: snapclientset,
		recorder:      nil,
	}

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Agent) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting open-local-storage-service agent")
	discoverer := discovery.NewDiscoverer(c.Configuration, c.kubeclientset, c.lssclientset, c.snapclientset)
	go wait.Until(discoverer.Discover, time.Duration(discoverer.DiscoverInterval)*time.Second, stopCh)
	go wait.Until(discoverer.InitResource, time.Duration(discoverer.DiscoverInterval)*time.Second, stopCh)

	// get auto expand snapshot interval
	var err error
	expandSnapInterval := discoverer.DiscoverInterval
	tmp := os.Getenv(lsstype.EnvExpandSnapInterval)
	if tmp != "" {
		expandSnapInterval, err = strconv.Atoi(tmp)
		if err != nil {
			return err
		}
	}
	go wait.Until(discoverer.ExpandSnapshotLVIfNeeded, time.Duration(expandSnapInterval)*time.Second, stopCh)

	klog.Info("Started open-local-storage-service agent")
	<-stopCh
	klog.Info("Shutting down agent")

	return nil
}