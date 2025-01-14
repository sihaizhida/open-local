/*
Copyright © 2021 Alibaba Group Holding Ltd.

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
	"context"
	"fmt"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	localv1alpha1 "github.com/alibaba/open-local/pkg/apis/storage/v1alpha1"
	clientset "github.com/alibaba/open-local/pkg/generated/clientset/versioned"
	"github.com/alibaba/open-local/pkg/generated/clientset/versioned/scheme"
	localscheme "github.com/alibaba/open-local/pkg/generated/clientset/versioned/scheme"
	informers "github.com/alibaba/open-local/pkg/generated/informers/externalversions/storage/v1alpha1"
	listers "github.com/alibaba/open-local/pkg/generated/listers/storage/v1alpha1"
)

const (
	SuccessSynced         = "Synced"
	MessageResourceSynced = "NLSC synced successfully"
)

type Controller struct {
	kubeclientset  kubernetes.Interface
	localclientset clientset.Interface

	nodesLister corelisters.NodeLister
	nodesSynced cache.InformerSynced
	nlsLister   listers.NodeLocalStorageLister
	nlsSynced   cache.InformerSynced
	nlscLister  listers.NodeLocalStorageInitConfigLister
	nlscSynced  cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder

	nlscName string
}

type WorkQueueItem struct {
	nlscName string
	nlsName  string
}

// NewController returns a new sample c
func NewController(
	kubeclientset kubernetes.Interface,
	localclientset clientset.Interface,
	nodeInformer coreinformers.NodeInformer,
	nlsInformer informers.NodeLocalStorageInformer,
	nlscInformer informers.NodeLocalStorageInitConfigInformer,
	nlscName string) *Controller {

	// Create event broadcaster
	utilruntime.Must(localscheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "open-local-controller"})

	c := &Controller{
		kubeclientset:  kubeclientset,
		localclientset: localclientset,
		nodesLister:    nodeInformer.Lister(),
		nodesSynced:    nodeInformer.Informer().HasSynced,
		nlsLister:      nlsInformer.Lister(),
		nlsSynced:      nlsInformer.Informer().HasSynced,
		nlscLister:     nlscInformer.Lister(),
		nlscSynced:     nlscInformer.Informer().HasSynced,
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NodeLocalStorageInitConfig"),
		recorder:       eventRecorder,
		nlscName:       nlscName,
	}

	log.Info("Setting up event handlers")
	nlscInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleNLSC,
		UpdateFunc: func(old, new interface{}) {
			c.handleNLSC(new)
		},
	})
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.createNLSByNode,
		DeleteFunc: c.deleteNLSByNode,
	})
	nlsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleNLS,
		UpdateFunc: func(old, new interface{}) {
			c.handleNLS(new)
		},
	})

	return c
}

func (c *Controller) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Wait for the caches to be synced before starting workers
	log.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.nlscSynced, c.nlsSynced, c.nodesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	log.Info("Starting controller workers")

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Info("Started controller")
	<-stopCh
	log.Info("Shutting down controller")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var item WorkQueueItem
		var ok bool
		if item, ok = obj.(WorkQueueItem); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected WorkQueueItem in workqueue but got %#v", obj))
			return nil
		}
		if err := c.syncHandler(item); err != nil {
			c.workqueue.AddRateLimited(item)
			return fmt.Errorf("error syncing '%#v': %s, requeuing", item, err.Error())
		}
		c.workqueue.Forget(obj)
		log.Debugf("Successfully synced '%#v'", item)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(item WorkQueueItem) error {
	// step 1: get nlsc
	nlsc, err := c.nlscLister.Get(item.nlscName)
	if err != nil {
		return err
	}

	// step 1: get nls name slice
	var nlsNames []string
	if item.nlsName != "" {
		nlsNames = append(nlsNames, item.nlsName)
	} else {
		nodelist, err := c.nodesLister.List(labels.Everything())
		if err != nil {
			return err
		}
		for _, node := range nodelist {
			nlsNames = append(nlsNames, node.Name)
		}
	}

	// step 2: handle
	for _, name := range nlsNames {
		nls, err := c.nlsLister.Get(name)

		// create nls if not found
		if errors.IsNotFound(err) {
			log.Warningf("nls %s not found", name)
			_, createErr := c.localclientset.CsiV1alpha1().NodeLocalStorages().Create(context.Background(), newNodeLocalStorage(name), metav1.CreateOptions{})
			if createErr != nil {
				return createErr
			}
			continue
		}
		if err != nil {
			return err
		}

		// update nls if needed
		if err := c.updateNLSIfNeeded(nls); err != nil {
			return err
		}
	}

	if item.nlsName == "" {
		c.recorder.Event(nlsc, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	}
	return nil
}

func newNodeLocalStorage(name string) *localv1alpha1.NodeLocalStorage {
	nls := new(localv1alpha1.NodeLocalStorage)
	nls.SetName(name)
	nls.Spec.NodeName = name

	return nls
}

func (c *Controller) updateNLSIfNeeded(nls *localv1alpha1.NodeLocalStorage) error {
	// update spec
	nlsc, err := c.nlscLister.Get(c.nlscName)
	if err != nil {
		return fmt.Errorf("get nlsc %s failed: %s", c.nlscName, err.Error())
	}

	node, err := c.nodesLister.Get(nls.Name)
	if err != nil {
		return fmt.Errorf("get node %s failed", nls.Name)
	}
	nodeLabels := node.Labels
	nlsCopy := nls.DeepCopy()
	nlsCopy.Spec.ListConfig = nlsc.Spec.GlobalConfig.ListConfig
	nlsCopy.Spec.ResourceToBeInited = nlsc.Spec.GlobalConfig.ResourceToBeInited
	for _, nodeconfig := range nlsc.Spec.NodesConfig {
		selector, err := metav1.LabelSelectorAsSelector(nodeconfig.Selector)
		if err != nil {
			return err
		}
		if !selector.Matches(labels.Set(nodeLabels)) {
			continue
		}
		nlsCopy.Spec.ListConfig = nodeconfig.ListConfig
		nlsCopy.Spec.ResourceToBeInited = nodeconfig.ResourceToBeInited
	}

	if !reflect.DeepEqual(nls, nlsCopy) {
		log.Infof("nls %s need to be updated", nls.Name)
		if _, err := c.localclientset.CsiV1alpha1().NodeLocalStorages().Update(context.Background(), nlsCopy, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) handleNLSC(obj interface{}) {
	var nlscName string
	var err error
	if nlscName, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.enqueueNLSC(nlscName, "")
}

func (c *Controller) handleNLS(obj interface{}) {
	var nlsName string
	var err error
	if nlsName, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.enqueueNLSC(c.nlscName, nlsName)
}

func (c *Controller) createNLSByNode(obj interface{}) {
	var nodeName string
	var err error
	if nodeName, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.enqueueNLSC(c.nlscName, nodeName)
}

func (c *Controller) deleteNLSByNode(obj interface{}) {
	var nodeName string
	var err error
	if nodeName, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	if err = c.localclientset.CsiV1alpha1().NodeLocalStorages().Delete(context.Background(), nodeName, *metav1.NewDeleteOptions(1)); err != nil {
		log.Errorf("Delete nls %s failed: %s", nodeName, err.Error())
	}
}

// if nlsName is "", then controller will iterate over all nls. It will be time consuming
func (c *Controller) enqueueNLSC(nlscName string, nlsName string) {
	if nlscName == c.nlscName {
		c.workqueue.Add(WorkQueueItem{
			nlscName: c.nlscName,
			nlsName:  nlsName,
		})
	}
}
