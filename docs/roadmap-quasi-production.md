# Roadmap Quasi Production

## Objectif

Faire evoluer Karasu d'un dashboard exploratoire vers un produit quasi production : fiable, lisible, testable, operable, et suffisamment robuste pour piloter une lecture quotidienne des opportunites.

## Definition de fini

Karasu est considere quasi production quand :

1. les opportunites sont classees par priorite metier plutot que par indicateurs bruts uniquement
2. chaque signal est explicable par un resume, des raisons, des risques, une convergence et une fraicheur
3. les erreurs de donnees et les retards de rafraichissement sont visibles
4. l'API expose des objets orientees decision reutilisables par plusieurs vues
5. les comportements critiques sont verrouilles par tests backend et validations frontend

## Roadmap par sprints

### Sprint 1 - Moteur d'opportunites backend

Statut : En cours

Objectif : creer une couche backend orientee decision sans casser l'API actuelle.

Livrables :

1. score de priorite backend combinant qualite, leader, convergence, fraicheur et momentum
2. resume d'opportunite avec bande de priorite et action primaire
3. endpoint `GET /api/opportunities`
4. tests sur le calcul de priorite et le tri des opportunites
5. documentation de la roadmap dans le repo

Definition of done :

1. le backend retourne une liste ordonnee d'opportunites
2. chaque opportunite contient un resume metier exploitable
3. les calculs sont couverts par tests

### Sprint 2 - Scanner operable frontend

Statut : En cours

Objectif : afficher les opportunites priorisees dans une vue scanner rapide.

Livrables :

1. panneau scanner alimente par `/api/opportunities`
2. filtres `fresh entries only`, `consensus only`, `priority band`
3. badges de fraicheur et de convergence dans la table principale
4. tri par priorite backend par defaut
5. messages UX encore plus orientees decision

Avancement actuel :

1. scanner d'opportunites priorisees branche au backend
2. filtres `fresh entries only`, `consensus only`, `priority band` disponibles
3. clic vers l'analyse detaillee d'un symbole depuis le scanner

Definition of done :

1. l'utilisateur peut reperer les opportunites prioritaires en quelques secondes
2. la table principale et le scanner restent coherents entre eux

### Sprint 3 - Alertes et observabilite

Statut : En cours

Objectif : rendre le systeme surveillable et moins dependant de la lecture manuelle.

Livrables :

1. moteur d'alertes dedoublonnees sur transitions critiques
2. historique local des alertes recentes
3. indicateurs de sante : fraicheur des donnees, dernier refresh, erreurs exchange
4. logs structures sur les calculs d'opportunites et erreurs de donnees

Avancement actuel :

1. endpoint `GET /api/system-health` expose la fraicheur live, l'etat du backfill et les retards 5m sur top symbols
2. panneau frontend Marches affiche un resume de sante systeme et les alertes principales
3. onglet Wallet enrichi avec un module d'aide a la decision (alleger / surveiller / renforcer) base sur les opportunites backend
4. moteur d'alertes dedoublonnees en memoire + endpoint `GET /api/alerts/recent`
5. affichage des alertes recentes dans la vue Marches
6. **onglet dédié Alertes** avec filtrage par severité (critique/alerte/info) et état (actif/resolu)
7. **vue complète de l'historique des alertes** avec tableau de bord de synthese (comptages actifs/resolus)

Definition of done :

1. un utilisateur comprend si le systeme est sain
2. les transitions majeures remontent sans bruit excessif

### Sprint 4 - Durcissement final

Statut : A faire

Objectif : fiabiliser l'experience et reduire le risque de regression.

Livrables :

1. tests d'integration backend sur les endpoints principaux
2. contrats de payload entre backend et frontend
3. gestion explicite des erreurs et des etats degradés
4. ajustements mobile et lisibilite finale

Definition of done :

1. les payloads stables sont verifies
2. les etats d'erreur sont lisibles dans l'UI
3. le produit reste exploitable sur desktop et mobile

## Notes de cadrage

1. la logique durable doit vivre cote backend
2. le front doit afficher et filtrer, pas recalculer le coeur metier
3. les signaux doivent rester prudents dans leur vocabulaire : opportunite, alignement, deterioration, pas de promesse implicite
