package models

import (
	"fmt"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/models/meshmodel/registry"
	regv1beta1 "github.com/meshery/meshkit/models/meshmodel/registry/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/component"
	"gorm.io/gorm"
)

// RegistryEnricher enriches MeshSync resources with Registry references
type RegistryEnricher struct {
	db              *gorm.DB
	registryManager *registry.RegistryManager

	// Cache to avoid repeated Registry lookups
	componentCache map[string]*componentCacheEntry
	cacheMutex     sync.RWMutex
}

// componentCacheEntry caches component and model IDs
type componentCacheEntry struct {
	ComponentID uuid.UUID
	ModelID     uuid.UUID
	ModelName   string
}

// NewRegistryEnricher creates a new enricher with cache
func NewRegistryEnricher(db *gorm.DB, regManager *registry.RegistryManager) *RegistryEnricher {
	return &RegistryEnricher{
		db:              db,
		registryManager: regManager,
		componentCache:  make(map[string]*componentCacheEntry),
	}
}

// getCacheKey creates a cache key from kind and apiVersion
func (e *RegistryEnricher) getCacheKey(kind, apiVersion string) string {
	return fmt.Sprintf("%s/%s", kind, apiVersion)
}

// EnrichResource adds ModelID and ComponentID by looking up Registry
func (e *RegistryEnricher) EnrichResource(resource *MeshSyncResource) error {
	// Skip if already enriched
	if resource.HasRegistryReferences() {
		return nil
	}

	// Skip if no kind or apiVersion (invalid resource)
	if resource.Kind == "" || resource.APIVersion == "" {
		return fmt.Errorf("resource missing kind or apiVersion")
	}

	// Check cache first
	cacheKey := e.getCacheKey(resource.Kind, resource.APIVersion)

	e.cacheMutex.RLock()
	cached, found := e.componentCache[cacheKey]
	e.cacheMutex.RUnlock()

	if found {
		// Use cached values
		resource.ComponentID = &cached.ComponentID
		resource.ModelID = &cached.ModelID
		if resource.Model == "" {
			resource.Model = cached.ModelName
		}
		return nil
	}

	// Look up in Registry
	componentID, modelID, modelName, err := e.findInRegistry(resource.Kind, resource.APIVersion)
	if err != nil {
		// Component not found in Registry - this is OK for custom resources
		// Log but continue (enrichment is optional)
		return nil
	}

	// Set Registry references
	resource.ComponentID = &componentID
	resource.ModelID = &modelID
	if resource.Model == "" {
		resource.Model = modelName
	}

	// Cache the result
	e.cacheMutex.Lock()
	e.componentCache[cacheKey] = &componentCacheEntry{
		ComponentID: componentID,
		ModelID:     modelID,
		ModelName:   modelName,
	}
	e.cacheMutex.Unlock()

	return nil
}

// findInRegistry searches Registry for matching component and returns IDs
func (e *RegistryEnricher) findInRegistry(kind, apiVersion string) (uuid.UUID, uuid.UUID, string, error) {
	// Query Registry for component using GetEntities with ComponentFilter
	// Note: ComponentFilter uses Name for the Kind field and APIVersion for version
	entities, _, _, err := e.registryManager.GetEntities(&regv1beta1.ComponentFilter{
		Name:       kind, // Kind is stored as Name in component
		APIVersion: apiVersion,
		Limit:      1, // Only need first match
	})

	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("failed to query registry: %w", err)
	}

	if len(entities) == 0 {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("component not found: kind=%s, apiVersion=%s", kind, apiVersion)
	}

	// Type assert to ComponentDefinition
	comp, ok := entities[0].(*component.ComponentDefinition)
	if !ok {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("entity is not a component")
	}

	// Get the model information from component (use Id not ID)
	modelID := comp.Model.Id
	modelName := comp.Model.Name
	componentID := comp.Id

	return componentID, modelID, modelName, nil
}

// EnrichAndSave enriches a resource and saves it to database
func (e *RegistryEnricher) EnrichAndSave(resource *MeshSyncResource) error {
	// Enrich with Registry references
	if err := e.EnrichResource(resource); err != nil {
		// Log warning but continue (enrichment is optional)
		fmt.Printf("Warning: Failed to enrich resource %s: %v\n", resource.ID, err)
	}

	// Save to database (GORM will cascade save related objects)
	return e.db.Save(resource).Error
}

// BatchEnrichAndSave enriches and saves multiple resources efficiently
func (e *RegistryEnricher) BatchEnrichAndSave(resources []*MeshSyncResource) error {
	// Use transaction for better performance
	return e.db.Transaction(func(tx *gorm.DB) error {
		for _, resource := range resources {
			// Enrich (uses cache for efficiency)
			if err := e.EnrichResource(resource); err != nil {
				fmt.Printf("Warning: Failed to enrich resource %s: %v\n", resource.ID, err)
			}

			// Save resource
			if err := tx.Save(resource).Error; err != nil {
				return fmt.Errorf("failed to save resource %s: %w", resource.ID, err)
			}
		}
		return nil
	})
}

// GetResourceWithComponent retrieves a resource and its component definition
func (e *RegistryEnricher) GetResourceWithComponent(resourceID string) (*MeshSyncResource, *component.ComponentDefinition, error) {
	var resource MeshSyncResource

	// Load resource with all relations
	if err := e.db.
		Preload("KubernetesResourceMeta").
		Preload("Spec").
		Preload("Status").
		First(&resource, "id = ?", resourceID).Error; err != nil {
		return nil, nil, err
	}

	// If no component reference, return resource only
	if resource.ComponentID == nil {
		return &resource, nil, nil
	}

	// Get component from Registry
	entities, _, _, err := e.registryManager.GetEntities(&regv1beta1.ComponentFilter{
		Id: resource.ComponentID.String(),
	})

	if err != nil || len(entities) == 0 {
		return &resource, nil, fmt.Errorf("component not found: %v", err)
	}

	// Type assert to ComponentDefinition
	comp, ok := entities[0].(*component.ComponentDefinition)
	if !ok {
		return &resource, nil, fmt.Errorf("entity is not a component")
	}

	return &resource, comp, nil
}

// ClearCache clears the component cache (useful for testing or after Registry updates)
func (e *RegistryEnricher) ClearCache() {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()
	e.componentCache = make(map[string]*componentCacheEntry)
}

// GetCacheSize returns the number of cached entries
func (e *RegistryEnricher) GetCacheSize() int {
	e.cacheMutex.RLock()
	defer e.cacheMutex.RUnlock()
	return len(e.componentCache)
}
