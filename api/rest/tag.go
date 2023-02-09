package apiv1

import DM "github.com/kycklingar/PBooru/DataManager"

type tagInterface interface {
	Parse(DM.Tag)
}

type tagString string

func (t *tagString) Parse(tag DM.Tag) {
	*t = tagString(tag.String())
}

type tag struct {
	Tag       string
	Namespace string
	Count     int
}

func (t *tag) Parse(Tag DM.Tag) {
	t.Tag = Tag.Tag
	t.Namespace = string(Tag.Namespace)
	t.Count = Tag.Count
}
