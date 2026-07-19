package auth

// Allowlist checks whether a Telegram user id is permitted to administer
// snagbox (ADMIN_TG_IDS).
type Allowlist struct {
	ids map[int64]struct{}
}

// NewAllowlist builds an Allowlist from a list of Telegram user ids.
func NewAllowlist(ids []int64) Allowlist {
	m := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return Allowlist{ids: m}
}

// Allowed reports whether tgID is an admin.
func (a Allowlist) Allowed(tgID int64) bool {
	_, ok := a.ids[tgID]
	return ok
}
