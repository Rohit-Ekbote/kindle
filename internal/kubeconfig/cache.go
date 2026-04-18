package kubeconfig

import (
	"bytes"
	"fmt"
	"sync"
)

type FetchFunc func(envName string) ([]byte, error)

type Cache struct {
	mu      sync.RWMutex
	entries map[string][]byte
	fetch   FetchFunc
}

func NewCache(fetch FetchFunc) *Cache {
	return &Cache{
		entries: make(map[string][]byte),
		fetch:   fetch,
	}
}

func (c *Cache) Get(envName string) ([]byte, error) {
	c.mu.RLock()
	data, ok := c.entries[envName]
	c.mu.RUnlock()
	if ok {
		return data, nil
	}

	data, err := c.fetch(envName)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[envName] = data
	c.mu.Unlock()
	return data, nil
}

func (c *Cache) Invalidate(envName string) {
	c.mu.Lock()
	delete(c.entries, envName)
	c.mu.Unlock()
}

// PatchServerURL replaces the 127.0.0.1:6443 server URL in a k3s kubeconfig with the env's DNS hostname.
// k3s kubeconfigs always use 127.0.0.1 as the server — we need the external FQDN for portal use.
func PatchServerURL(raw []byte, fqdn string) ([]byte, error) {
	old := []byte("https://127.0.0.1:6443")
	newURL := []byte(fmt.Sprintf("https://%s:6443", fqdn))
	if !bytes.Contains(raw, old) {
		return nil, fmt.Errorf("expected server URL %s not found in kubeconfig", old)
	}
	return bytes.ReplaceAll(raw, old, newURL), nil
}
