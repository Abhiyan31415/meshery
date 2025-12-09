package models

import (
    "github.com/gofrs/uuid"
    meshsyncmodel "github.com/meshery/meshsync/pkg/model"
    "gorm.io/gorm"
)

// MeshSyncResource extends MeshSync's KubernetesResource with Registry references
// This allows us to reference Registry instead of duplicating component metadata
type MeshSyncResource struct {
    meshsyncmodel.KubernetesResource // Embed the base MeshSync model
    ModelID     *uuid.UUID `json:"model_id,omitempty" gorm:"type:uuid;index:idx_registry_refs,priority:1;index:idx_model_component,priority:1"`
    ComponentID *uuid.UUID `json:"component_id,omitempty" gorm:"type:uuid;index:idx_registry_refs,priority:2;index:idx_model_component,priority:2"`
}
// Add this function to the file:

// CreateMeshSyncIndexes creates database indexes for efficient querying
func CreateMeshSyncIndexes(db *gorm.DB) error {
    indexes := []string{
        // Composite index for Registry enrichment lookups
        `CREATE INDEX IF NOT EXISTS idx_k8s_resources_kind_apiversion 
         ON kubernetes_resources(kind, api_version)`,
        
        // Index on model_id for Registry joins
        `CREATE INDEX IF NOT EXISTS idx_k8s_resources_model_id 
         ON kubernetes_resources(model_id) WHERE model_id IS NOT NULL`,
        
        // Index on component_id for Registry joins
        `CREATE INDEX IF NOT EXISTS idx_k8s_resources_component_id 
         ON kubernetes_resources(component_id) WHERE component_id IS NOT NULL`,
        
        // Composite index for cluster-scoped queries
        `CREATE INDEX IF NOT EXISTS idx_k8s_resources_cluster_model 
         ON kubernetes_resources(cluster_id, model_id)`,
        
        // Index on cluster_id for delete operations
        `CREATE INDEX IF NOT EXISTS idx_k8s_resources_cluster_id 
         ON kubernetes_resources(cluster_id)`,
    }
    
    for _, indexSQL := range indexes {
        if err := db.Exec(indexSQL).Error; err != nil {
            return err
        }
    }
    
    return nil
}
// TableName overrides the table name to use the same table as MeshSync
func (MeshSyncResource) TableName() string {
    return "kubernetes_resources"
}

// BeforeCreate hook - generates ID if not set and handles embedded struct hooks
func (m *MeshSyncResource) BeforeCreate(tx *gorm.DB) error {
    // Call the embedded struct's BeforeCreate if it exists
    if m.ID == "" {
        meshsyncmodel.SetID(&m.KubernetesResource)
    }
    return nil
}

// BeforeSave hook - handles embedded struct hooks
func (m *MeshSyncResource) BeforeSave(tx *gorm.DB) error {
    // Call the embedded struct's BeforeSave if it exists
    if m.ID == "" {
        meshsyncmodel.SetID(&m.KubernetesResource)
    }
    return nil
}

// IsObject checks if the resource has valid metadata
func (m *MeshSyncResource) IsObject() bool {
    return meshsyncmodel.IsObject(m.KubernetesResource)
}

// GetClusterID returns the cluster ID as string
func (m *MeshSyncResource) GetClusterID() string {
    return m.ClusterID
}

// GetKind returns the resource kind
func (m *MeshSyncResource) GetKind() string {
    return m.Kind
}

// GetAPIVersion returns the API version
func (m *MeshSyncResource) GetAPIVersion() string {
    return m.APIVersion
}

// GetName returns the resource name from metadata
func (m *MeshSyncResource) GetName() string {
    if m.KubernetesResourceMeta != nil {
        return m.KubernetesResourceMeta.Name
    }
    return ""
}

// GetNamespace returns the resource namespace from metadata
func (m *MeshSyncResource) GetNamespace() string {
    if m.KubernetesResourceMeta != nil {
        return m.KubernetesResourceMeta.Namespace
    }
    return ""
}

// HasRegistryReferences checks if the resource has been enriched with Registry refs
func (m *MeshSyncResource) HasRegistryReferences() bool {
    return m.ModelID != nil && m.ComponentID != nil
}

// NeedsEnrichment checks if the resource needs Registry enrichment
func (m *MeshSyncResource) NeedsEnrichment() bool {
    // Needs enrichment if it has kind/apiVersion but no Registry references
    return m.Kind != "" && m.APIVersion != "" && !m.HasRegistryReferences()
}