export function formatBarsAgo(value: number): string {
  if (value < 0) {
    return 'non observe recemment'
  }

  if (value === 0) {
    return 'sur cette bougie'
  }

  if (value === 1) {
    return 'il y a 1 bougie'
  }

  return `il y a ${value} bougies`
}

export function translateProfileLabel(label: string): string {
  switch (label) {
    case 'Intraday Momentum':
      return 'Momentum intraday'
    case 'Swing Balance':
      return 'Swing equilibre'
    case 'Trend Follow':
      return 'Suivi de tendance'
    default:
      return label
  }
}

export function translateStateLabel(state: string): string {
  switch (state) {
    case 'entry':
      return 'entree'
    case 'hold':
      return 'maintien'
    case 'watch':
      return 'surveillance'
    case 'exit':
      return 'sortie'
    case 'avoid':
      return 'a eviter'
    default:
      return state
  }
}

export function translatePriorityBand(band: string): string {
  switch (band) {
    case 'actionable':
      return 'actionnable'
    case 'strong-watch':
      return 'surveillance forte'
    case 'watchlist':
      return 'liste de surveillance'
    case 'defensive':
      return 'defensif'
    default:
      return band
  }
}

export function translatePrimaryAction(action: string): string {
  switch (action) {
    case 'act-now':
      return 'agir maintenant'
    case 'watch-closely':
      return 'surveiller de pres'
    case 'prepare':
      return 'preparer'
    case 'avoid':
      return 'eviter'
    default:
      return action
  }
}

export function translateOpportunitySummary(summary: string): string {
  switch (summary) {
    case 'Fresh multi-profile entry detected':
      return 'Nouvelle entree detectee avec alignement de plusieurs profils'
    case 'Fresh entry led by the strongest profile':
      return 'Nouvelle entree portee par le profil le plus fort'
    case 'Recent exit transition requires caution':
      return 'Une sortie recente impose de la prudence'
    case 'High-conviction alignment remains active':
      return 'L alignement a forte conviction reste actif'
    case 'Constructive setup worth close monitoring':
      return 'Configuration constructive a surveiller de pres'
    case 'Setup is building but not fully confirmed':
      return 'La configuration se met en place mais reste a confirmer'
    case 'Context remains defensive or low conviction':
      return 'Le contexte reste defensif ou peu convaincant'
    default:
      return summary
  }
}

export function translateReason(reason: string): string {
  switch (reason) {
    case 'multi-profile alignment active':
      return 'alignement multi-profils actif'
    case 'multiple profiles remain constructive':
      return 'plusieurs profils restent constructifs'
    case 'recent entry transition detected':
      return 'transition recente vers entree detectee'
    case 'momentum confirmed':
      return 'momentum confirme'
    case 'recent price thrust matches profile':
      return 'l acceleration recente des prix correspond au profil'
    case 'macd momentum confirmed':
      return 'momentum MACD confirme'
    case 'volume participation supportive':
      return 'le volume soutient le mouvement'
    case 'trend structure aligned':
      return 'la structure de tendance est alignee'
    case 'price location remains tradable':
      return 'la position du prix reste exploitable'
    case 'rsi balance is constructive':
      return 'l equilibre RSI est constructif'
    default:
      return reason
  }
}

export function translateRisk(risk: string): string {
  switch (risk) {
    case 'recent exit transition detected':
      return 'transition recente vers sortie detectee'
    case 'insufficient history':
      return 'historique insuffisant'
    case 'recent price thrust is insufficient':
      return 'l acceleration recente des prix est insuffisante'
    case 'macd momentum too weak':
      return 'le momentum MACD est trop faible'
    case 'volume confirmation missing':
      return 'la confirmation par le volume manque'
    case 'trend structure below profile floor':
      return 'la structure de tendance est sous le seuil du profil'
    case 'price location too stretched or weak':
      return 'la position du prix est trop etiree ou trop faible'
    case 'rsi balance is weak':
      return 'l equilibre RSI est faible'
    default:
      return risk
  }
}

export function translateHealthIssue(issue: string): string {
  if (issue === 'flux live 1m stale') {
    return 'flux live 1m en retard'
  }
  return issue
}

export function translateAlertSeverity(severity: string): string {
  switch (severity) {
    case 'critical':
      return 'critique'
    case 'warning':
      return 'alerte'
    case 'info':
      return 'info'
    default:
      return severity
  }
}
