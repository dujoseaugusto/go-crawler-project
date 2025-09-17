package crawler

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
)

// PatternStorage gerencia a persistência de padrões aprendidos
type PatternStorage struct {
	storageDir string
	logger     *logger.Logger
	mutex      sync.RWMutex
}

// NewPatternStorage cria um novo gerenciador de armazenamento de padrões
func NewPatternStorage(storageDir string) *PatternStorage {
	if storageDir == "" {
		storageDir = "./data/patterns"
	}

	// Cria diretório se não existir
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		logger.NewLogger("pattern_storage").WithFields(map[string]interface{}{"error": err}).Warn("Failed to create storage directory")
	}

	return &PatternStorage{
		storageDir: storageDir,
		logger:     logger.NewLogger("pattern_storage"),
	}
}

// SaveContentPatterns salva padrões de conteúdo no disco
func (ps *PatternStorage) SaveContentPatterns(learner *ContentBasedPatternLearner) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	data, err := learner.ExportPatterns()
	if err != nil {
		return fmt.Errorf("failed to export patterns: %w", err)
	}

	filename := filepath.Join(ps.storageDir, "content_patterns.json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write patterns file: %w", err)
	}

	ps.logger.WithField("file", filename).Info("Content patterns saved to disk")
	return nil
}

// LoadContentPatterns carrega padrões de conteúdo do disco
func (ps *PatternStorage) LoadContentPatterns(learner *ContentBasedPatternLearner) error {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	filename := filepath.Join(ps.storageDir, "content_patterns.json")

	// Verifica se o arquivo existe
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		ps.logger.WithField("file", filename).Info("No existing patterns file found")
		return nil // Não é erro, apenas não há padrões salvos
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read patterns file: %w", err)
	}

	if err := learner.ImportPatterns(data); err != nil {
		return fmt.Errorf("failed to import patterns: %w", err)
	}

	ps.logger.WithField("file", filename).Info("Content patterns loaded from disk")
	return nil
}

// SaveURLPatterns salva padrões de URL no disco
func (ps *PatternStorage) SaveURLPatterns(learner *PatternLearner) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	data, err := learner.ExportPatterns()
	if err != nil {
		return fmt.Errorf("failed to export URL patterns: %w", err)
	}

	filename := filepath.Join(ps.storageDir, "url_patterns.json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write URL patterns file: %w", err)
	}

	ps.logger.WithField("file", filename).Info("URL patterns saved to disk")
	return nil
}

// LoadURLPatterns carrega padrões de URL do disco
func (ps *PatternStorage) LoadURLPatterns(learner *PatternLearner) error {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	filename := filepath.Join(ps.storageDir, "url_patterns.json")

	// Verifica se o arquivo existe
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		ps.logger.WithField("file", filename).Info("No existing URL patterns file found")
		return nil // Não é erro, apenas não há padrões salvos
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read URL patterns file: %w", err)
	}

	if err := learner.ImportPatterns(data); err != nil {
		return fmt.Errorf("failed to import URL patterns: %w", err)
	}

	ps.logger.WithField("file", filename).Info("URL patterns loaded from disk")
	return nil
}

// AutoSaveContentPatterns salva padrões automaticamente em intervalos
func (ps *PatternStorage) AutoSaveContentPatterns(learner *ContentBasedPatternLearner, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := ps.SaveContentPatterns(learner); err != nil {
				ps.logger.Error("Failed to auto-save content patterns", err)
			}
		}
	}()
}

// GetPatternsInfo retorna informações sobre os padrões salvos
func (ps *PatternStorage) GetPatternsInfo() map[string]interface{} {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	info := make(map[string]interface{})

	// Verifica arquivo de padrões de conteúdo
	contentFile := filepath.Join(ps.storageDir, "content_patterns.json")
	if stat, err := os.Stat(contentFile); err == nil {
		info["content_patterns"] = map[string]interface{}{
			"exists":      true,
			"size":        stat.Size(),
			"modified_at": stat.ModTime(),
		}
	} else {
		info["content_patterns"] = map[string]interface{}{
			"exists": false,
		}
	}

	// Verifica arquivo de padrões de URL
	urlFile := filepath.Join(ps.storageDir, "url_patterns.json")
	if stat, err := os.Stat(urlFile); err == nil {
		info["url_patterns"] = map[string]interface{}{
			"exists":      true,
			"size":        stat.Size(),
			"modified_at": stat.ModTime(),
		}
	} else {
		info["url_patterns"] = map[string]interface{}{
			"exists": false,
		}
	}

	info["storage_dir"] = ps.storageDir
	return info
}

// SharedContentLearner é uma instância global compartilhada
var (
	sharedContentLearner *ContentBasedPatternLearner
	sharedPatternStorage *PatternStorage
	sharedLearnerMutex   sync.Once
)

// GetSharedContentLearner retorna a instância compartilhada do ContentBasedPatternLearner
func GetSharedContentLearner() *ContentBasedPatternLearner {
	sharedLearnerMutex.Do(func() {
		sharedContentLearner = NewContentBasedPatternLearner()
		sharedPatternStorage = NewPatternStorage("./data/patterns")

		// Carrega padrões existentes
		if err := sharedPatternStorage.LoadContentPatterns(sharedContentLearner); err != nil {
			logger.NewLogger("shared_content_learner").WithFields(map[string]interface{}{"error": err}).Warn("Failed to load existing patterns")
		}

		// Auto-save a cada 5 minutos
		sharedPatternStorage.AutoSaveContentPatterns(sharedContentLearner, 5*time.Minute)

		logger.NewLogger("shared_content_learner").Info("Shared ContentBasedPatternLearner initialized with persistent storage")
	})

	return sharedContentLearner
}

// SaveSharedPatterns força o salvamento dos padrões compartilhados
func SaveSharedPatterns() error {
	if sharedContentLearner == nil || sharedPatternStorage == nil {
		return fmt.Errorf("shared learner not initialized")
	}

	return sharedPatternStorage.SaveContentPatterns(sharedContentLearner)
}

// GetSharedPatternsInfo retorna informações sobre os padrões compartilhados
func GetSharedPatternsInfo() map[string]interface{} {
	if sharedPatternStorage == nil {
		return map[string]interface{}{
			"initialized": false,
		}
	}

	info := sharedPatternStorage.GetPatternsInfo()
	info["initialized"] = true
	return info
}
