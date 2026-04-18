package presets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var ErrPresetNotFound = errors.New("preset not found")

type Loader struct {
	dir string
}

func NewLoader(dir string) *Loader {
	return &Loader{dir: dir}
}

func (l *Loader) List() ([]*Preset, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, fmt.Errorf("read presets dir: %w", err)
	}
	var presets []*Preset
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := l.load(e.Name())
		if err != nil {
			return nil, fmt.Errorf("load preset %s: %w", e.Name(), err)
		}
		presets = append(presets, p)
	}
	return presets, nil
}

func (l *Loader) Get(name string) (*Preset, error) {
	p, err := l.load(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrPresetNotFound
	}
	return p, err
}

func (l *Loader) load(name string) (*Preset, error) {
	path := filepath.Join(l.dir, name, "template.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Preset
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	p.dir = filepath.Join(l.dir, name)
	return &p, nil
}

func (p *Preset) BaseValues() ([]byte, error) {
	return os.ReadFile(filepath.Join(p.dir, "values.yaml"))
}
