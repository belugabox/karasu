import { useState } from 'react'
import { TabNav } from './components/TabNav'
import { AlertsPanel } from './components/AlertsPanel'
import { TasksPanel } from './components/TasksPanel'
import { TopMarketsPanel } from './components/TopMarketsPanel'
import { WalletPanel } from './components/WalletPanel'
import './App.css'
type AppTab = 'wallet' | 'markets' | 'alerts' | 'tasks'

function App() {
  const [activeTab, setActiveTab] = useState<AppTab>('wallet')

  return (
    <main className="app-shell">
      <header className="topbar">
        <p className="eyebrow">Tableau de bord Karasu</p>
        <h1>Portefeuille, Marches, Alertes, Taches</h1>
      </header>

      <TabNav activeTab={activeTab} onChange={setActiveTab} />

      {activeTab === 'wallet' && <WalletPanel />}
      {activeTab === 'markets' && <TopMarketsPanel />}
      {activeTab === 'alerts' && <AlertsPanel />}
      {activeTab === 'tasks' && <TasksPanel />}
    </main>
  )
}

export default App
