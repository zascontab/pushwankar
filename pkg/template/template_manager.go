package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"sync"
	"text/template"

	"notification-service/pkg/logging"
)

// Template representa una plantilla de notificación
type Template struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Title       string               `json:"title"`
	Body        string               `json:"body"`
	Data        map[string]string    `json:"data,omitempty"`
	Locales     map[string]Localized `json:"locales,omitempty"`
}

// Localized representa una versión localizada de una plantilla
type Localized struct {
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data,omitempty"`
}

// TemplateManager gestiona las plantillas de notificaciones
type TemplateManager struct {
	templates      map[string]*template.Template
	templateData   map[string]Template
	defaultLocale  string
	templateFolder string
	mu             sync.RWMutex
	logger         *logging.Logger
}

// NewTemplateManager crea una nueva instancia de TemplateManager
func NewTemplateManager(templateFolder, defaultLocale string, logger *logging.Logger) *TemplateManager {
	return &TemplateManager{
		templates:      make(map[string]*template.Template),
		templateData:   make(map[string]Template),
		defaultLocale:  defaultLocale,
		templateFolder: templateFolder,
		logger:         logger,
	}
}

// LoadTemplates carga todas las plantillas desde un directorio
func (m *TemplateManager) LoadTemplates() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Limpiar plantillas existentes
	m.templates = make(map[string]*template.Template)
	m.templateData = make(map[string]Template)

	// Listar archivos JSON en el directorio
	pattern := filepath.Join(m.templateFolder, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		m.logger.Warn("No template files found in %s", m.templateFolder)
		return nil
	}

	// Cargar cada archivo de plantilla
	for _, file := range files {
		if err := m.loadTemplateFile(file); err != nil {
			m.logger.Error("Error loading template file %s: %v", file, err)
		}
	}

	m.logger.Info("Loaded %d notification templates", len(m.templates))
	return nil
}

// loadTemplateFile carga una plantilla desde un archivo
func (m *TemplateManager) loadTemplateFile(filePath string) error {
	// Leer archivo
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Decodificar JSON
	var tmpl Template
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return err
	}

	// Validar ID
	if tmpl.ID == "" {
		return errors.New("template ID cannot be empty")
	}

	// Compilar plantilla de título
	titleTmpl, err := template.New(tmpl.ID + "_title").Parse(tmpl.Title)
	if err != nil {
		return err
	}

	// Compilar plantilla de cuerpo
	bodyTmpl, err := template.New(tmpl.ID + "_body").Parse(tmpl.Body)
	if err != nil {
		return err
	}

	// Combinar las plantillas
	combinedTmpl := template.New(tmpl.ID)
	combinedTmpl.AddParseTree(titleTmpl.Name(), titleTmpl.Tree)
	combinedTmpl.AddParseTree(bodyTmpl.Name(), bodyTmpl.Tree)

	// Procesar las versiones localizadas
	for locale, localized := range tmpl.Locales {
		if localized.Title != "" {
			localizedTitleTmpl, err := template.New(tmpl.ID + "_" + locale + "_title").Parse(localized.Title)
			if err != nil {
				m.logger.Error("Error parsing localized title template for locale %s: %v", locale, err)
				continue
			}
			combinedTmpl.AddParseTree(localizedTitleTmpl.Name(), localizedTitleTmpl.Tree)
		}

		if localized.Body != "" {
			localizedBodyTmpl, err := template.New(tmpl.ID + "_" + locale + "_body").Parse(localized.Body)
			if err != nil {
				m.logger.Error("Error parsing localized body template for locale %s: %v", locale, err)
				continue
			}
			combinedTmpl.AddParseTree(localizedBodyTmpl.Name(), localizedBodyTmpl.Tree)
		}
	}

	// Almacenar la plantilla
	m.templates[tmpl.ID] = combinedTmpl
	m.templateData[tmpl.ID] = tmpl

	return nil
}

// GetTemplate obtiene una plantilla por su ID
func (m *TemplateManager) GetTemplate(templateID string) (Template, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tmpl, exists := m.templateData[templateID]
	return tmpl, exists
}

// RenderTemplate renderiza una plantilla con los datos proporcionados
func (m *TemplateManager) RenderTemplate(templateID, locale string, data map[string]interface{}) (string, string, map[string]string, error) {
	m.mu.RLock()
	tmpl, exists := m.templates[templateID]
	templateData, _ := m.templateData[templateID]
	m.mu.RUnlock()

	if !exists {
		return "", "", nil, errors.New("template not found")
	}

	// Determinar qué nombre de plantilla usar según el locale
	titleTemplate := templateID + "_title"
	bodyTemplate := templateID + "_body"

	// Verificar si existe una versión localizada
	localizedTitle := templateID + "_" + locale + "_title"
	localizedBody := templateID + "_" + locale + "_body"

	// Si existe una versión localizada para el título, usarla
	if localizedTmpl := tmpl.Lookup(localizedTitle); localizedTmpl != nil {
		titleTemplate = localizedTitle
	}

	// Si existe una versión localizada para el cuerpo, usarla
	if localizedTmpl := tmpl.Lookup(localizedBody); localizedTmpl != nil {
		bodyTemplate = localizedBody
	}

	// Renderizar título
	var titleBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&titleBuf, titleTemplate, data); err != nil {
		return "", "", nil, err
	}

	// Renderizar cuerpo
	var bodyBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&bodyBuf, bodyTemplate, data); err != nil {
		return "", "", nil, err
	}

	// Obtener datos adicionales
	extraData := templateData.Data
	if locale != "" && locale != m.defaultLocale {
		if localized, exists := templateData.Locales[locale]; exists && localized.Data != nil {
			// Combinar datos adicionales localizados
			mergedData := make(map[string]string)
			for k, v := range extraData {
				mergedData[k] = v
			}
			for k, v := range localized.Data {
				mergedData[k] = v
			}
			extraData = mergedData
		}
	}

	return titleBuf.String(), bodyBuf.String(), extraData, nil
}

// AddTemplate agrega una nueva plantilla (útil para pruebas o actualización dinámica)
func (m *TemplateManager) AddTemplate(tmpl Template) error {
	// Validar campos requeridos
	if tmpl.ID == "" {
		return errors.New("template ID cannot be empty")
	}
	if tmpl.Title == "" || tmpl.Body == "" {
		return errors.New("template title and body cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Compilar plantillas
	titleTmpl, err := template.New(tmpl.ID + "_title").Parse(tmpl.Title)
	if err != nil {
		return err
	}

	bodyTmpl, err := template.New(tmpl.ID + "_body").Parse(tmpl.Body)
	if err != nil {
		return err
	}

	// Combinar las plantillas
	combinedTmpl := template.New(tmpl.ID)
	combinedTmpl.AddParseTree(titleTmpl.Name(), titleTmpl.Tree)
	combinedTmpl.AddParseTree(bodyTmpl.Name(), bodyTmpl.Tree)

	// Procesar las versiones localizadas
	for locale, localized := range tmpl.Locales {
		if localized.Title != "" {
			localizedTitleTmpl, err := template.New(tmpl.ID + "_" + locale + "_title").Parse(localized.Title)
			if err != nil {
				m.logger.Error("Error parsing localized title template for locale %s: %v", locale, err)
				continue
			}
			combinedTmpl.AddParseTree(localizedTitleTmpl.Name(), localizedTitleTmpl.Tree)
		}

		if localized.Body != "" {
			localizedBodyTmpl, err := template.New(tmpl.ID + "_" + locale + "_body").Parse(localized.Body)
			if err != nil {
				m.logger.Error("Error parsing localized body template for locale %s: %v", locale, err)
				continue
			}
			combinedTmpl.AddParseTree(localizedBodyTmpl.Name(), localizedBodyTmpl.Tree)
		}
	}

	// Almacenar la plantilla
	m.templates[tmpl.ID] = combinedTmpl
	m.templateData[tmpl.ID] = tmpl

	return nil
}

// RemoveTemplate elimina una plantilla existente
func (m *TemplateManager) RemoveTemplate(templateID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.templates[templateID]; exists {
		delete(m.templates, templateID)
		delete(m.templateData, templateID)
		return true
	}
	return false
}

// GetAllTemplates devuelve todas las plantillas disponibles
func (m *TemplateManager) GetAllTemplates() []Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]Template, 0, len(m.templateData))
	for _, tmpl := range m.templateData {
		templates = append(templates, tmpl)
	}

	return templates
}
