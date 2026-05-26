import { useState } from 'react'
import { TabNav } from './components/TabNav'
import { TopMarketsPanel } from './components/TopMarketsPanel'
import { WalletPanel } from './components/WalletPanel'
import { SystemPanel } from './components/SystemPanel'
import './App.css'
type AppTab = 'wallet' | 'markets' | 'system'

function App() {
  const [activeTab, setActiveTab] = useState<AppTab>('wallet')

  return (
    <main className="app-shell">
      <header className="topbar">
        <div className="topbar-logo">🐦‍⬛✨</div>
        <h1>Karasu</h1>
        <p className="topbar-sub">Tableau de bord crypto</p>
      </header>

      <TabNav activeTab={activeTab} onChange={setActiveTab} />

      {activeTab === 'wallet' && <WalletPanel />}
      {activeTab === 'markets' && <TopMarketsPanel />}
      {activeTab === 'system' && <SystemPanel />}
    </main>
  )
}

export default App
