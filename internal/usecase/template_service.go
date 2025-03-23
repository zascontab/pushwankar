package usecase

import (
	"errors"
	"fmt"

	"notification-service/pkg/template"
)

// TemplateService define las operaciones de negocio para gestionar plantillas de notificaciones
type TemplateService struct {
	templateManager *template.TemplateManager
}

// NewTemplateService crea una nueva instancia de TemplateService
func NewTemplateService(templateManager *template.TemplateManager) *TemplateService {
	return &TemplateService{
		templateManager: templateManager,
	}
}

// RenderTemplate renderiza una plantilla con los datos proporcionados
func (s *TemplateService) RenderTemplate(templateID, locale string, data map[string]interface{}) (string, string, map[string]string, error) {
	if templateID == "" {
		return "", "", nil, errors.New("template ID cannot be empty")
	}

	// Validar los datos requeridos según la plantilla
	_, exists := s.templateManager.GetTemplate(templateID)
	if !exists {
		return "", "", nil, fmt.Errorf("template '%s' not found", templateID)
	}

	// Renderizar la plantilla
	title, body, extraData, err := s.templateManager.RenderTemplate(templateID, locale, data)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to render template: %w", err)
	}

	return title, body, extraData, nil
}

// GetTemplates obtiene todas las plantillas disponibles
func (s *TemplateService) GetTemplates() []template.Template {
	return s.templateManager.GetAllTemplates()
}

// GetTemplate obtiene una plantilla específica
func (s *TemplateService) GetTemplate(templateID string) (template.Template, error) {
	tmpl, exists := s.templateManager.GetTemplate(templateID)
	if !exists {
		return template.Template{}, fmt.Errorf("template '%s' not found", templateID)
	}
	return tmpl, nil
}

// CreateTemplate crea una nueva plantilla
func (s *TemplateService) CreateTemplate(tmpl template.Template) error {
	if tmpl.ID == "" || tmpl.Title == "" || tmpl.Body == "" {
		return errors.New("template ID, title and body are required")
	}

	return s.templateManager.AddTemplate(tmpl)
}

// UpdateTemplate actualiza una plantilla existente
func (s *TemplateService) UpdateTemplate(tmpl template.Template) error {
	if tmpl.ID == "" {
		return errors.New("template ID is required")
	}

	// Verificar si la plantilla existe
	_, exists := s.templateManager.GetTemplate(tmpl.ID)
	if !exists {
		return fmt.Errorf("template '%s' not found", tmpl.ID)
	}

	// Eliminar la plantilla existente
	s.templateManager.RemoveTemplate(tmpl.ID)

	// Agregar la plantilla actualizada
	return s.templateManager.AddTemplate(tmpl)
}

// DeleteTemplate elimina una plantilla
func (s *TemplateService) DeleteTemplate(templateID string) error {
	if !s.templateManager.RemoveTemplate(templateID) {
		return fmt.Errorf("template '%s' not found", templateID)
	}
	return nil
}

// RenderNotificationFromTemplate renderiza una notificación completa a partir de una plantilla
func (s *TemplateService) RenderNotificationFromTemplate(
	templateID string,
	locale string,
	data map[string]interface{},
) (title string, body string, notificationData map[string]interface{}, err error) {

	// Renderizar la plantilla
	title, body, extraData, err := s.RenderTemplate(templateID, locale, data)
	if err != nil {
		return "", "", nil, err
	}

	// Combinar datos extra con los datos proporcionados
	if data == nil {
		data = make(map[string]interface{})
	}

	notificationData = make(map[string]interface{})
	for k, v := range data {
		notificationData[k] = v
	}

	// Agregar datos de la plantilla
	if extraData != nil {
		for k, v := range extraData {
			notificationData[k] = v
		}
	}

	// Agregar información de la plantilla
	notificationData["template_id"] = templateID
	if locale != "" {
		notificationData["locale"] = locale
	}

	return title, body, notificationData, nil
}
