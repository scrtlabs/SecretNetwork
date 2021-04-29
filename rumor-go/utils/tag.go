package utils

import "strings"

type Tag struct {
	tagMap map[string]interface{}
}

func NewTagMap(tags string, ok bool) (*Tag, bool) {
	if !ok {
		return nil, ok
	}

	tagSplit := strings.Split(tags, ",")
	tagMap := map[string]interface{}{}
	for _, tag := range tagSplit {
		tagMap[tag] = true
	}

	return &Tag{tagMap: tagMap}, true
}

func (tag Tag) Option(name string) (interface{}, bool) {
	v, o := tag.tagMap[name]
	return v, o
}
