# CSD DevTrack - Instructions pour Claude

## Logging

**Toujours utiliser le logger intégré** au lieu de `fmt.Printf`, `log.Printf`, ou `println`.

### Import
```go
import "csd-devtrack/cli/modules/platform/logger"
```

### Utilisation
```go
logger.Debug("Message de debug: %s", value)
logger.Info("Information: %s", value)
logger.Warn("Attention: %s", value)
logger.Error("Erreur: %s", value)
```

### Pourquoi ?
- Les logs sont centralisés et formatés uniformément
- Les logs sont diffusés aux clients TUI connectés via WebSocket
- Les logs sont écrits dans le fichier `~/.csd-devtrack/daemon.log`
- Le niveau de log est configurable

### Exceptions
- Les modules qui créeraient un cycle d'import avec `logger` (comme `ui/core`) peuvent utiliser `log` standard
- Dans ce cas, préférer ne pas logger ou utiliser un callback

### Debug temporaire
- **Même pour du debug temporaire**, utiliser `logger.Debug()` au lieu de `fmt.Printf()`
- Les logs de debug vont dans `~/.csd-devtrack/daemon.log`
- Utiliser `tail -f ~/.csd-devtrack/daemon.log` pour suivre les logs

## Architecture

### MVP Pattern
- **Model** (`platform/*/model.go`) - Structures de données
- **View** (`ui/tui/`) - Rendu terminal avec Bubble Tea
- **Presenter** (`ui/core/presenter.go`) - Logique métier et état

### Services
- Chaque service est dans `modules/platform/<service>/`
- Les services sont initialisés dans `presenter.Initialize()`

### Capabilities
- Module `platform/capabilities/` détecte les outils externes
- Utilisé pour cacher les fonctionnalités si prérequis manquants
- Outils détectés: tmux, claude, psql, mysql, sqlite3, git, go, node, npm
