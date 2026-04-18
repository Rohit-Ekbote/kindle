package presets

type Preset struct {
	Name         string             `yaml:"name"`
	Description  string             `yaml:"description"`
	Defaults     PresetDefaults     `yaml:"defaults"`
	ImageTagKeys map[string]string  `yaml:"image_tag_keys"`
	UserEditable []UserEditableField `yaml:"user_editable"`
	dir          string
}

type PresetDefaults struct {
	MachineType string `yaml:"machine_type"`
	DiskSizeGB  int64  `yaml:"disk_size_gb"`
	Zone        string `yaml:"zone"`
	ChartRef    string `yaml:"chart_ref"`
}

type UserEditableField struct {
	Key     string   `yaml:"key"`
	Label   string   `yaml:"label"`
	Type    string   `yaml:"type"` // enum | integer | string | boolean
	Options []string `yaml:"options,omitempty"`
	Min     *int64   `yaml:"min,omitempty"`
	Max     *int64   `yaml:"max,omitempty"`
	Default any      `yaml:"default,omitempty"`
}
