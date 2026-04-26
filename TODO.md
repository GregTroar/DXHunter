# FlexDXCluster — TODO / Idées d'amélioration

## FTx Tab

- [x] Countdown timer visuel jusqu'à la prochaine période TX/RX (15 s FT8 / 7.5 s FT4 / 3.25 s FT2)
- [ ] Log des QSO FTx effectués via autocall (callsign, bande, mode, heure) avec export ADIF
- [ ] Mode Fox/Hound : détecter les décodes `F/H` et adapter l'autocall en conséquence
- [ ] Alerte sonore quand une station de la watchlist est décodée
- [x] Statistiques autocall : taux de réussite, temps moyen par QSO

## Cluster / Spots

- [ ] Filtre bande dans la table des spots (clic sur 20m pour n'afficher que 20m)
- [ ] Indicateur d'âge visuel des spots (barre ou couleur qui s'estompe avec le temps)
- [ ] Données de propagation en temps réel dans la sidebar : SFI, A-index, K-index (API DXHeat ou HamQTH)
- [x] Greyline indicator — visualiser si une bande est ouverte vers une direction donnée
- [ ] Poster un spot manuellement (formulaire DX / fréquence / mode / commentaire)

## Interface générale

- [x] UI de configuration : modifier `config.yml` depuis l'interface sans éditer le fichier à la main
- [ ] Vue carte (Leaflet) : positionner les spots sur une carte avec les grilles Maidenhead
- [ ] Raccourcis clavier (ex : Espace = tune, H = Halt TX, etc.)
- [x] Thème clair / sombre commutable
- [ ] Ajouter un pop up au premier demarrage pour creer le fichier config ainsi que demander les informations principales necessaires au fonctionnement du logiciel (Call, Locator, Flex, Log4OM Db etc..),par defaut 2 clusters: f4bpo.cluster.com 7300 (Master) et le pota-cluster.iz2lsc.eu 7373, bien sur si config non detecte ne rien lancer (flex, log4om, cluster etc..)

## Notifications

- [ ] Alerte sonore configurable par type d'événement (nouveau DXCC, watchlist, mon indicatif spotté)
- [ ] Webhook Discord / Telegram en complément de Gotify

## Log / Intégration

- [ ] LoTW status directement sur les spots (colonne ou icône — API ARRL LoTW)
- [ ] Logging direct depuis l'app (sans dépendre de Log4OM) pour les sessions FTx
- [ ] Export ADIF du log
