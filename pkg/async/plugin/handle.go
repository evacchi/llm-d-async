/*
Copyright 2026 The llm-d-incubation Authors.

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

package plugin

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	fwkplugin "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
)

// This is lifted from EppHandle in GIE

// Handle provides plugins a set of standard data and tools to work with
type Handle interface {
	// Context returns a context the plugins can use, if they need one
	Context() context.Context

	HandlePlugins

	// PodList lists pods.
	PodList() []types.NamespacedName
}

// HandlePlugins defines a set of APIs to work with instantiated plugins
type HandlePlugins interface {
	// Plugin returns the named plugin instance
	Plugin(name string) fwkplugin.Plugin

	// AddPlugin adds a plugin to the set of known plugin instances
	AddPlugin(name string, plugin fwkplugin.Plugin)

	// GetAllPlugins returns all of the known plugins
	GetAllPlugins() []fwkplugin.Plugin

	// GetAllPluginsWithNames returns all of the known plugins with their names
	GetAllPluginsWithNames() map[string]fwkplugin.Plugin
}

// PodListFunc is a function type that filters and returns a list of pod metrics
type PodListFunc func() []types.NamespacedName

// asyncHandle is an implementation of the interface plugins.Handle
type asyncHandle struct {
	ctx context.Context
	HandlePlugins
	podList PodListFunc
}

// Context returns a context the plugins can use, if they need one
func (h *asyncHandle) Context() context.Context {
	return h.ctx
}

// asyncHandlePlugins implements the set of APIs to work with instantiated plugins
type asyncHandlePlugins struct {
	plugins map[string]fwkplugin.Plugin
}

// Plugin returns the named plugin instance
func (h *asyncHandlePlugins) Plugin(name string) fwkplugin.Plugin {
	return h.plugins[name]
}

// AddPlugin adds a plugin to the set of known plugin instances
func (h *asyncHandlePlugins) AddPlugin(name string, plugin fwkplugin.Plugin) {
	h.plugins[name] = plugin
}

// GetAllPlugins returns all of the known plugins
func (h *asyncHandlePlugins) GetAllPlugins() []fwkplugin.Plugin {
	result := make([]fwkplugin.Plugin, 0)
	for _, plugin := range h.plugins {
		result = append(result, plugin)
	}
	return result
}

// GetAllPluginsWithNames returns al of the known plugins with their names
func (h *asyncHandlePlugins) GetAllPluginsWithNames() map[string]fwkplugin.Plugin {
	return h.plugins
}

// PodList lists pods.
func (h *asyncHandle) PodList() []types.NamespacedName {
	return h.podList()
}

func NewAsyncHandle(ctx context.Context, podList PodListFunc) Handle {
	return &asyncHandle{
		ctx: ctx,
		HandlePlugins: &asyncHandlePlugins{
			plugins: map[string]fwkplugin.Plugin{},
		},
		podList: podList,
	}
}
