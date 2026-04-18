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
	presets := make([]*Preset, 0)
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
		return nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	p.dir = filepath.Join(l.dir, name)
	return &p, nil
}

func (p *Preset) BaseValues() ([]byte, error) {
	if p.dir == "" {
		return nil, errors.New("preset has no directory set")
	}
	data, err := os.ReadFile(filepath.Join(p.dir, "values.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read values for preset %s: %w", p.Name, err)
	}
	return data, nil
}
