package registry

type scopedCell struct {
	cell  *cell
	write bool
}

type Scoped struct {
	scope map[string]*scopedCell
}

func (s *Scoped) GetAny(key string) (any, bool) {
	if s.scope == nil {
		return nil, false
	}

	if c, ex := s.scope[key]; ex {
		if c.cell == nil {
			return nil, false
		}
		return c.cell.value, true
	}

	return nil, false
}

func (s *Scoped) SetAny(key string, value any) bool {
	if s.scope == nil {
		return false
	}

	if c, ex := s.scope[key]; ex {
		if c.cell != nil && c.write {
			c.cell.value = value
			return true
		}
	}
	return false
}
