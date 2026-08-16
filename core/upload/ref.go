package upload

import (
	"database/sql/driver"
	"fmt"
	"path"
	"strings"
)

const (
	StagedPrefix = "staged"
	MediaPrefix  = "media"
	TrashPrefix  = "trash"
)

type Ref string

func (r Ref) String() string { return string(r) }

func (r Ref) Empty() bool { return strings.TrimSpace(string(r)) == "" }

func (r Ref) Staged() bool { return strings.HasPrefix(string(r), StagedPrefix+"/") }

func (r Ref) Committed() bool { return strings.HasPrefix(string(r), MediaPrefix+"/") }

func (r Ref) Extension() string { return path.Ext(string(r)) }

func (r Ref) Digest() string {
	name := path.Base(string(r))
	if dot := strings.IndexByte(name, '.'); dot > 0 {
		return name[:dot]
	}
	return name
}

func (r Ref) promoted() Ref {
	return Ref(MediaPrefix + "/" + strings.TrimPrefix(string(r), StagedPrefix+"/"))
}

func (r Ref) trashed() Ref {
	return Ref(TrashPrefix + "/" + strings.TrimPrefix(string(r), MediaPrefix+"/"))
}

func (r Ref) Value() (driver.Value, error) {
	if r.Empty() {
		return "", nil
	}
	return string(r), nil
}

func (r *Ref) Scan(value any) error {
	switch typed := value.(type) {
	case nil:
		*r = ""
	case string:
		*r = Ref(typed)
	case []byte:
		*r = Ref(typed)
	default:
		return fmt.Errorf("coyote/upload: cannot read a Ref from %T", value)
	}
	return nil
}

func (Ref) GormDataType() string { return "string" }
