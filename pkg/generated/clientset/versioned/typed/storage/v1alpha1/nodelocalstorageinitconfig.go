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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/oecp/open-local-storage-service/pkg/apis/storage/v1alpha1"
	scheme "github.com/oecp/open-local-storage-service/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// NodeLocalStorageInitConfigsGetter has a method to return a NodeLocalStorageInitConfigInterface.
// A group's client should implement this interface.
type NodeLocalStorageInitConfigsGetter interface {
	NodeLocalStorageInitConfigs() NodeLocalStorageInitConfigInterface
}

// NodeLocalStorageInitConfigInterface has methods to work with NodeLocalStorageInitConfig resources.
type NodeLocalStorageInitConfigInterface interface {
	Create(ctx context.Context, nodeLocalStorageInitConfig *v1alpha1.NodeLocalStorageInitConfig, opts v1.CreateOptions) (*v1alpha1.NodeLocalStorageInitConfig, error)
	Update(ctx context.Context, nodeLocalStorageInitConfig *v1alpha1.NodeLocalStorageInitConfig, opts v1.UpdateOptions) (*v1alpha1.NodeLocalStorageInitConfig, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.NodeLocalStorageInitConfig, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.NodeLocalStorageInitConfigList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.NodeLocalStorageInitConfig, err error)
	NodeLocalStorageInitConfigExpansion
}

// nodeLocalStorageInitConfigs implements NodeLocalStorageInitConfigInterface
type nodeLocalStorageInitConfigs struct {
	client rest.Interface
}

// newNodeLocalStorageInitConfigs returns a NodeLocalStorageInitConfigs
func newNodeLocalStorageInitConfigs(c *StorageV1alpha1Client) *nodeLocalStorageInitConfigs {
	return &nodeLocalStorageInitConfigs{
		client: c.RESTClient(),
	}
}

// Get takes name of the nodeLocalStorageInitConfig, and returns the corresponding nodeLocalStorageInitConfig object, and an error if there is any.
func (c *nodeLocalStorageInitConfigs) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.NodeLocalStorageInitConfig, err error) {
	result = &v1alpha1.NodeLocalStorageInitConfig{}
	err = c.client.Get().
		Resource("nodelocalstorageinitconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of NodeLocalStorageInitConfigs that match those selectors.
func (c *nodeLocalStorageInitConfigs) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.NodeLocalStorageInitConfigList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.NodeLocalStorageInitConfigList{}
	err = c.client.Get().
		Resource("nodelocalstorageinitconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested nodeLocalStorageInitConfigs.
func (c *nodeLocalStorageInitConfigs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("nodelocalstorageinitconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a nodeLocalStorageInitConfig and creates it.  Returns the server's representation of the nodeLocalStorageInitConfig, and an error, if there is any.
func (c *nodeLocalStorageInitConfigs) Create(ctx context.Context, nodeLocalStorageInitConfig *v1alpha1.NodeLocalStorageInitConfig, opts v1.CreateOptions) (result *v1alpha1.NodeLocalStorageInitConfig, err error) {
	result = &v1alpha1.NodeLocalStorageInitConfig{}
	err = c.client.Post().
		Resource("nodelocalstorageinitconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(nodeLocalStorageInitConfig).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a nodeLocalStorageInitConfig and updates it. Returns the server's representation of the nodeLocalStorageInitConfig, and an error, if there is any.
func (c *nodeLocalStorageInitConfigs) Update(ctx context.Context, nodeLocalStorageInitConfig *v1alpha1.NodeLocalStorageInitConfig, opts v1.UpdateOptions) (result *v1alpha1.NodeLocalStorageInitConfig, err error) {
	result = &v1alpha1.NodeLocalStorageInitConfig{}
	err = c.client.Put().
		Resource("nodelocalstorageinitconfigs").
		Name(nodeLocalStorageInitConfig.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(nodeLocalStorageInitConfig).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the nodeLocalStorageInitConfig and deletes it. Returns an error if one occurs.
func (c *nodeLocalStorageInitConfigs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("nodelocalstorageinitconfigs").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *nodeLocalStorageInitConfigs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("nodelocalstorageinitconfigs").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched nodeLocalStorageInitConfig.
func (c *nodeLocalStorageInitConfigs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.NodeLocalStorageInitConfig, err error) {
	result = &v1alpha1.NodeLocalStorageInitConfig{}
	err = c.client.Patch(pt).
		Resource("nodelocalstorageinitconfigs").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}