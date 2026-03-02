package parse

import (
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var timeLayouts = []string{
	"2006-01-02 15:04:05", // updatedAt: 2026-02-28 18:38:25
	"2006-01-02",          // createdAt: 2026-02-05
	time.RFC3339,
}

type yamlTime struct {
	t time.Time
}

func (y *yamlTime) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return nil
	}
	s := strings.TrimSpace(n.Value)
	if s == "" {
		return nil
	}
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			y.t = t
			return nil
		}
	}
	return nil
}

type yamlAliases struct {
	values []string
}

func (y *yamlAliases) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		y.values = []string{n.Value}
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		for _, c := range n.Content {
			if c.Kind == yaml.ScalarNode {
				y.values = append(y.values, c.Value)
			}
		}
		return nil
	}
	return nil
}

func (y *yamlAliases) Values() []string {
	if y == nil {
		return nil
	}
	return y.values
}

type yamlTags struct {
	values []string
}

func (y *yamlTags) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		y.values = []string{n.Value}
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		for _, c := range n.Content {
			if c.Kind == yaml.ScalarNode {
				y.values = append(y.values, c.Value)
			}
		}
		return nil
	}
	return nil
}

func (y *yamlTags) Values() []string {
	if y == nil {
		return nil
	}
	return y.values
}
