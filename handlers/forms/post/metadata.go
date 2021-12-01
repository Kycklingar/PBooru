package postform

import (
	"strings"
	"time"

	DM "github.com/kycklingar/PBooru/DataManager"
)

const iso8601 = "2006-01-02"

func processMetadata(ua *DM.UserActions, postID int, metaStr string) error {
	for _, m := range strings.Split(metaStr, "\n") {
		data := strings.SplitN(strings.TrimSpace(m), ":", 2)
		if len(data) < 2 {
			continue
		}
		namespace, value := data[0], data[1]

		if namespace == "date" {
			t, err := time.Parse(iso8601, value)
			if err != nil {
				return err
			}

			ua.Add(DM.PostAddCreationDate(postID, t))
		} else {
			ua.Add(DM.PostAddMetaData(postID, namespace, value))
		}
	}

	return nil
}

func processMetaRemoval(ua *DM.UserActions, postID int, metaData []string) error {
	for _, m := range metaData {
		data := strings.SplitN(m, ":", 2)
		if len(data) < 2 {
			continue
		}

		namespace, value := data[0], data[1]

		if namespace == "date" {
			t, err := time.Parse(iso8601, value)
			if err != nil {
				return err
			}
			ua.Add(DM.PostRemoveCreationDate(postID, t))
		} else {
			ua.Add(DM.PostRemoveMetaData(postID, namespace, value))
		}
	}

	return nil
}
