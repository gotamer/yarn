package internal

import (
	"encoding/json"
	"fmt"
	"strings"

	sync "github.com/sasha-s/go-deadlock"
	"gopkg.in/yaml.v2"
)

type FeatureType int

const (
	// FeatureInvalid is the invalid feature (0)
	FeatureInvalid FeatureType = iota
)

func (f FeatureType) String() string {
	switch f {
	default:
		return "invalid_feature"
	}
}

func AvailableFeatures() []string {
	features := []string{}
	for i := 1; i <= 99; i++ {
		feature := FeatureType(i)
		if feature.String() == FeatureInvalid.String() {
			break
		}
		features = append(features, feature.String())
	}
	return features
}

func FeatureFromString(s string) (FeatureType, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	default:
		fs := fmt.Sprintf("available features: %s", strings.Join(AvailableFeatures(), " "))
		return FeatureInvalid, fmt.Errorf("Error unrecognised feature: %s (%s)", s, fs)
	}
}

func FeaturesFromStrings(xs []string) ([]FeatureType, error) {
	var features []FeatureType

	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}

		f, err := FeatureFromString(x)
		if err != nil {
			return nil, err
		}
		features = append(features, f)
	}

	return features, nil
}

// FeatureFlags describes a set of Pods optional Features
// and whether they are enabled or disabled
type FeatureFlags struct {
	sync.RWMutex
	flags map[FeatureType]bool
}

func (f *FeatureFlags) reset() {
	f.flags = make(map[FeatureType]bool)
}

func (f *FeatureFlags) Reset() {
	f.Lock()
	defer f.Unlock()

	f.reset()
}

func (f FeatureFlags) AsStrings() []string {
	var vs []string
	for flag := range f.flags {
		vs = append(vs, flag.String())
	}
	return vs
}

func (f FeatureFlags) String() string {
	return strings.Join(f.AsStrings(), " ")
}

func (f *FeatureFlags) Disable(feature FeatureType) {
	f.Lock()
	defer f.Unlock()

	if f.flags == nil {
		f.reset()
	}

	f.flags[feature] = false
}

func (f *FeatureFlags) DisableAll(features []FeatureType) {
	f.Lock()
	defer f.Unlock()

	if f.flags == nil {
		f.reset()
	}

	for _, feature := range features {
		f.flags[feature] = false
	}
}

func (f *FeatureFlags) Enable(feature FeatureType) {
	f.Lock()
	defer f.Unlock()

	if f.flags == nil {
		f.reset()
	}

	f.flags[feature] = true
}

func (f *FeatureFlags) EnableAll(features []FeatureType) {
	f.Lock()
	defer f.Unlock()

	if f.flags == nil {
		f.reset()
	}

	for _, feature := range features {
		f.flags[feature] = true
	}
}

func (f *FeatureFlags) IsEnabled(feature FeatureType) bool {
	f.RLock()
	defer f.RUnlock()

	return f.flags[feature]
}

func (f FeatureFlags) MarshalJSON() ([]byte, error) {
	var vs []FeatureType
	for flag := range f.flags {
		vs = append(vs, flag)
	}
	return json.Marshal(vs)
}

func (f *FeatureFlags) UnmarshalJSON(b []byte) error {
	var vs []FeatureType
	if err := json.Unmarshal(b, &vs); err != nil {
		return err
	}
	f.flags = make(map[FeatureType]bool)
	for _, v := range vs {
		f.flags[v] = true
	}
	return nil
}

func (f FeatureFlags) MarshalYAML() ([]byte, error) {
	var vs []FeatureType
	for flag := range f.flags {
		vs = append(vs, flag)
	}
	return yaml.Marshal(vs)
}

func (f *FeatureFlags) UnmarshalYAML(b []byte) error {
	var vs []FeatureType
	if err := yaml.Unmarshal(b, &vs); err != nil {
		return err
	}
	f.flags = make(map[FeatureType]bool)
	for _, v := range vs {
		f.flags[v] = true
	}
	return nil
}

// WithEnabledFeatures enables the selected features
func WithEnabledFeatures(features []FeatureType) Option {
	return func(cfg *Config) error {
		cfg.Features.Reset()
		cfg.Features.EnableAll(features)
		return nil
	}
}
