package style

import (
	"errors"
	"fmt"
)

type CellRef struct {
	Sheet string
	Cell  string
}

type FieldType string

const (
	FieldTypeNumber FieldType = "number"
	FieldTypeText   FieldType = "text"
)

type FieldConfig struct {
	Name string
	Cell CellRef
	Type FieldType
}

type Config struct {
	ID           string
	Name         string
	TemplatePath string
	TemplateMark CellRef
	TemplateID   string
	Result       CellRef
	Fields       map[string]FieldConfig
}

type Registry struct {
	byID   map[string]Config
	byName map[string]string
}

func NewRegistry(configs []Config) (*Registry, error) {
	if len(configs) == 0 {
		return nil, errors.New("style registry requires at least one style")
	}

	registry := &Registry{
		byID:   make(map[string]Config, len(configs)),
		byName: make(map[string]string, len(configs)),
	}
	for _, cfg := range configs {
		if err := validateConfig(cfg); err != nil {
			return nil, err
		}
		if _, exists := registry.byID[cfg.ID]; exists {
			return nil, fmt.Errorf("duplicate style id %q", cfg.ID)
		}
		if _, exists := registry.byName[cfg.Name]; exists {
			return nil, fmt.Errorf("duplicate style name %q", cfg.Name)
		}
		registry.byID[cfg.ID] = cfg
		registry.byName[cfg.Name] = cfg.ID
	}

	return registry, nil
}

func (r *Registry) List() []Config {
	styles := make([]Config, 0, len(r.byID))
	for _, cfg := range r.byID {
		styles = append(styles, cfg)
	}
	return styles
}

func (r *Registry) GetByID(id string) (Config, bool) {
	cfg, ok := r.byID[id]
	return cfg, ok
}

func (r *Registry) GetByName(name string) (Config, bool) {
	id, ok := r.byName[name]
	if !ok {
		return Config{}, false
	}
	return r.GetByID(id)
}

func (r *Registry) MustResolveNames(names []string) ([]Config, error) {
	styles := make([]Config, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		cfg, ok := r.GetByName(name)
		if !ok {
			return nil, fmt.Errorf("unknown style %q", name)
		}
		if _, exists := seen[cfg.ID]; exists {
			continue
		}
		seen[cfg.ID] = struct{}{}
		styles = append(styles, cfg)
	}
	return styles, nil
}

func validateConfig(cfg Config) error {
	if cfg.ID == "" {
		return errors.New("style id is required")
	}
	if cfg.Name == "" {
		return fmt.Errorf("style %q name is required", cfg.ID)
	}
	if cfg.TemplatePath == "" {
		return fmt.Errorf("style %q template path is required", cfg.ID)
	}
	if cfg.Result.Sheet == "" || cfg.Result.Cell == "" {
		return fmt.Errorf("style %q result cell is required", cfg.ID)
	}
	for name, field := range cfg.Fields {
		if name == "" || field.Name == "" {
			return fmt.Errorf("style %q has empty field name", cfg.ID)
		}
		if name != field.Name {
			return fmt.Errorf("style %q field key %q must match original field name %q", cfg.ID, name, field.Name)
		}
		if field.Cell.Sheet == "" || field.Cell.Cell == "" {
			return fmt.Errorf("style %q field %q cell is required", cfg.ID, name)
		}
	}
	return nil
}
