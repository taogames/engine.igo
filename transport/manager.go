package transport

type Manager struct {
	m map[string]Transport

	upgradeList []string
}

func NewManager(ts []Transport) *Manager {
	manager := &Manager{
		m:           make(map[string]Transport),
		upgradeList: make([]string, len(ts)),
	}
	for i, t := range ts {
		manager.m[t.Name()] = t
		manager.upgradeList[i] = t.Name()
	}

	return manager
}

func (m *Manager) Get(name string) (Transport, bool) {
	t, ok := m.m[name]
	return t, ok
}

func (m *Manager) Upgradable(name string) []string {
	for i, v := range m.upgradeList {
		if v == name {
			return m.upgradeList[i+1:]
		}
	}

	return []string{}
}

func (m *Manager) CanUpgrade(from, to string) bool {
	for i := 0; i < len(m.upgradeList) && m.upgradeList[i] != from; i++ {
		if m.upgradeList[i] == to {
			return false
		}
	}
	return true
}
