package diff

type VersionedDiff struct {
	Version int64 `json:"version"`

	Old string `json:"old"`

	New string `json:"new"`
}
