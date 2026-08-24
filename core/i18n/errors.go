package i18n

import "errors"

var (
	ErrNoCatalog     = errors.New("coyote/i18n: no catalog for that locale")
	ErrMalformedPO   = errors.New("coyote/i18n: malformed PO file")
	ErrBadPluralRule = errors.New("coyote/i18n: Plural-Forms is not a rule this can evaluate")
)
