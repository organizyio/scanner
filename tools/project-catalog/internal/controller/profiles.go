//go:build cgo

package controller

import (
	"path/filepath"
	"sort"

	"scan-projects/internal/storage"
)

type Profiles struct {
	path   string
	config storage.AppConfig
}

func NewProfiles() *Profiles {
	p := &Profiles{
		path: storage.SettingsPath(),
	}
	p.loadOrInit()
	return p
}

func (p *Profiles) loadOrInit() {
	cfg, err := storage.LoadYAML[storage.AppConfig](p.path)
	if err != nil || cfg.Profiles == nil || len(cfg.Profiles) == 0 {
		cp := filepath.Join(storage.AppDataDir(), "output", "checkpoint.json")
		cfg = storage.DefaultConfig(cp)
		_ = storage.SaveYAML(p.path, cfg)
	}
	_ = cfg.EnsureValid()
	p.config = cfg
	_ = storage.SaveYAML(p.path, p.config)
}

func (p *Profiles) Save() { _ = storage.SaveYAML(p.path, p.config) }

func (p *Profiles) Names() []string {
	names := make([]string, 0, len(p.config.Profiles))
	for k := range p.config.Profiles {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func (p *Profiles) ActiveName() string { return p.config.ActiveProfile }

func (p *Profiles) SetActive(name string) bool {
	if _, ok := p.config.Profiles[name]; !ok {
		return false
	}
	p.config.ActiveProfile = name
	p.Save()
	return true
}

func (p *Profiles) Active() storage.ScanProfile {
	return p.config.Profiles[p.config.ActiveProfile]
}

func (p *Profiles) UpdateActive(profile storage.ScanProfile) {
	key := p.config.ActiveProfile
	profile.Name = key
	p.config.Profiles[key] = profile
	p.Save()
}

func (p *Profiles) New(name string) bool {
	if name == "" {
		return false
	}
	if _, ok := p.config.Profiles[name]; ok {
		return false
	}
	base := p.Active()
	base.Name = name
	p.config.Profiles[name] = base
	p.config.ActiveProfile = name
	p.Save()
	return true
}

func (p *Profiles) Delete(name string) bool {
	if len(p.config.Profiles) <= 1 {
		return false
	}
	if _, ok := p.config.Profiles[name]; !ok {
		return false
	}
	delete(p.config.Profiles, name)
	if p.config.ActiveProfile == name {
		for k := range p.config.Profiles {
			p.config.ActiveProfile = k
			break
		}
	}
	p.Save()
	return true
}
