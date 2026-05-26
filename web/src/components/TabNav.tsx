type AppTab = 'wallet' | 'markets' | 'alerts' | 'tasks'

type TabNavProps = {
  activeTab: AppTab
  onChange: (tab: AppTab) => void
}

const tabs: Array<{ id: AppTab; label: string }> = [
  { id: 'wallet', label: 'Portefeuille' },
  { id: 'markets', label: 'Marches' },
  { id: 'alerts', label: 'Alertes' },
  { id: 'tasks', label: 'Taches' },
]

export function TabNav({ activeTab, onChange }: TabNavProps) {
  return (
    <nav className="tabs" aria-label="Onglets principaux">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          className={activeTab === tab.id ? 'tab-button active' : 'tab-button'}
          onClick={() => onChange(tab.id)}
        >
          {tab.label}
        </button>
      ))}
    </nav>
  )
}
