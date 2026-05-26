type AppTab = 'wallet' | 'markets'

type TabNavProps = {
  activeTab: AppTab
  onChange: (tab: AppTab) => void
}

const tabs: Array<{ id: AppTab; label: string; emoji: string }> = [
  { id: 'wallet', label: 'Portefeuille', emoji: '💜' },
  { id: 'markets', label: 'Marchés', emoji: '🌸' },
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
          {tab.emoji} {tab.label}
        </button>
      ))}
    </nav>
  )
}
