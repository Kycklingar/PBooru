package postform

import (
	"net/url"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
)

// Form keys
const (
	postid          = "post-id"
	metadata        = "post-metadata"
	metadataRemove  = "post-metadata-remove"
	description     = "post-description"
	descriptionDiff = "post-description-diff"
	creationdate    = "post-creationdate"
	tags            = "post-tags"
	tagsDiff        = "post-tags-diff"
)

func ProcessFormData(ua *DM.UserActions, form url.Values) error {
	postID, err := strconv.Atoi(form.Get(postid))
	if err != nil {
		return err
	}

	meta := form.Get(metadata)
	if len(meta) > 0 {
		metalogs, err := DM.PostAddMetaData(postID, meta)
		if err != nil {
			return err
		}

		ua.Add(metalogs...)
	}

	metaRem := form[metadataRemove]
	if len(metaRem) > 0 {
		metalogs, err := DM.PostRemoveMetaData(postID, metaRem)
		if err != nil {
			return err
		}

		ua.Add(metalogs...)
	}

	descr := form.Get(description)

	if descr != form.Get(descriptionDiff) {
		ua.Add(DM.PostChangeDescription(postID, descr))
	}

	tagstr := form.Get(tags)
	tagdiff := form.Get(tagsDiff)
	if tagstr != tagdiff {
		ua.Add(DM.AlterPostTags(postID, tagstr, tagdiff))
	}

	return nil
}
