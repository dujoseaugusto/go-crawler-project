package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel representa os níveis de log
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String retorna a representação string do nível de log
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry representa uma entrada de log estruturada
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Component string                 `json:"component,omitempty"`
	Function  string                 `json:"function,omitempty"`
	File      string                 `json:"file,omitempty"`
	Line      int                    `json:"line,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Logger é o logger estruturado customizado
type Logger struct {
	level     LogLevel
	component string
	fields    map[string]interface{}
}

// NewLogger cria um novo logger
func NewLogger(component string) *Logger {
	return &Logger{
		level:     INFO, // Nível padrão
		component: component,
		fields:    make(map[string]interface{}),
	}
}

// SetLevel define o nível mínimo de log
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// WithField adiciona um campo ao contexto do logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		level:     l.level,
		component: l.component,
		fields:    make(map[string]interface{}),
	}

	// Copia campos existentes
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Adiciona novo campo
	newLogger.fields[key] = value

	return newLogger
}

// WithFields adiciona múltiplos campos ao contexto do logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := &Logger{
		level:     l.level,
		component: l.component,
		fields:    make(map[string]interface{}),
	}

	// Copia campos existentes
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Adiciona novos campos
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithError adiciona um erro ao contexto do logger
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// log é o método interno que faz o log estruturado
func (l *Logger) log(level LogLevel, message string, err error) {
	if level < l.level {
		return
	}

	// Captura informações do caller
	_, file, line, ok := runtime.Caller(2)
	var funcName string
	var fileName string

	if ok {
		// Extrai apenas o nome do arquivo
		parts := strings.Split(file, "/")
		if len(parts) > 0 {
			fileName = parts[len(parts)-1]
		}

		// Captura o nome da função
		pc, _, _, ok := runtime.Caller(2)
		if ok {
			fn := runtime.FuncForPC(pc)
			if fn != nil {
				funcName = fn.Name()
				// Remove o path do package, mantém apenas o nome da função
				if idx := strings.LastIndex(funcName, "."); idx != -1 {
					funcName = funcName[idx+1:]
				}
			}
		}
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Message:   message,
		Component: l.component,
		Function:  funcName,
		File:      fileName,
		Line:      line,
		Fields:    l.fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	// Serializa para JSON
	jsonBytes, jsonErr := json.Marshal(entry)
	if jsonErr != nil {
		// Fallback para log simples se JSON falhar
		log.Printf("[%s] %s: %s (JSON error: %v)", level.String(), l.component, message, jsonErr)
		return
	}

	// Imprime o log estruturado
	fmt.Println(string(jsonBytes))

	// Se for FATAL, termina o programa
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug registra uma mensagem de debug
func (l *Logger) Debug(message string) {
	l.log(DEBUG, message, nil)
}

// Debugf registra uma mensagem de debug formatada
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...), nil)
}

// Info registra uma mensagem informativa
func (l *Logger) Info(message string) {
	l.log(INFO, message, nil)
}

// Infof registra uma mensagem informativa formatada
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, args...), nil)
}

// Warn registra uma mensagem de aviso
func (l *Logger) Warn(message string) {
	l.log(WARN, message, nil)
}

// Warnf registra uma mensagem de aviso formatada
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, args...), nil)
}

// Error registra uma mensagem de erro
func (l *Logger) Error(message string, err error) {
	l.log(ERROR, message, err)
}

// Errorf registra uma mensagem de erro formatada
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...), nil)
}

// Fatal registra uma mensagem fatal e termina o programa
func (l *Logger) Fatal(message string, err error) {
	l.log(FATAL, message, err)
}

// Fatalf registra uma mensagem fatal formatada e termina o programa
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, args...), nil)
}

// Logger global padrão
var defaultLogger = NewLogger("app")

// Funções globais para conveniência
func Debug(message string) {
	defaultLogger.Debug(message)
}

func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

func Info(message string) {
	defaultLogger.Info(message)
}

func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

func Warn(message string) {
	defaultLogger.Warn(message)
}

func Warnf(format string, args ...interface{}) {
	defaultLogger.Warnf(format, args...)
}

func Error(message string, err error) {
	defaultLogger.Error(message, err)
}

func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

func Fatal(message string, err error) {
	defaultLogger.Fatal(message, err)
}

func Fatalf(format string, args ...interface{}) {
	defaultLogger.Fatalf(format, args...)
}

func WithField(key string, value interface{}) *Logger {
	return defaultLogger.WithField(key, value)
}

func WithFields(fields map[string]interface{}) *Logger {
	return defaultLogger.WithFields(fields)
}

func WithError(err error) *Logger {
	return defaultLogger.WithError(err)
}

func SetLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}
