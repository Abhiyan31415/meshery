package models

import (
    "github.com/gofrs/uuid"
    "gorm.io/gorm"
)

// DeleteResourcesByCluster deletes all MeshSync resources for a given cluster
// This replaces the old 5-table delete operations with efficient cascading deletes
func DeleteResourcesByCluster(db *gorm.DB, clusterID uuid.UUID) error {
    // Single delete operation - GORM handles cascading deletes for related tables
    return db.Where("cluster_id = ?", clusterID.String()).
        Delete(&MeshSyncResource{}).Error
}

// DeleteResourcesByClusterString is a convenience wrapper for string cluster IDs
func DeleteResourcesByClusterString(db *gorm.DB, clusterID string) error {
    return db.Where("cluster_id = ?", clusterID).
        Delete(&MeshSyncResource{}).Error
}

// GetResourcesByComponent gets all resources of a specific component type
func GetResourcesByComponent(db *gorm.DB, componentID uuid.UUID) ([]*MeshSyncResource, error) {
    var resources []*MeshSyncResource
    err := db.Where("component_id = ?", componentID).
        Preload("KubernetesResourceMeta").
        Preload("Spec").
        Preload("Status").
        Find(&resources).Error
    return resources, err
}

// GetResourcesByModel gets all resources of a specific model
func GetResourcesByModel(db *gorm.DB, modelID uuid.UUID) ([]*MeshSyncResource, error) {
    var resources []*MeshSyncResource
    err := db.Where("model_id = ?", modelID).
        Preload("KubernetesResourceMeta").
        Preload("Spec").
        Preload("Status").
        Find(&resources).Error
    return resources, err
}

// GetResourcesByClusterAndKind gets resources by cluster and kind (efficient query using indexes)
func GetResourcesByClusterAndKind(db *gorm.DB, clusterID string, kind string) ([]*MeshSyncResource, error) {
    var resources []*MeshSyncResource
    err := db.Where("cluster_id = ? AND kind = ?", clusterID, kind).
        Preload("KubernetesResourceMeta").
        Preload("Spec").
        Preload("Status").
        Find(&resources).Error
    return resources, err
}

// GetResourcesByClusterAndComponent gets resources by cluster and component (most efficient)
func GetResourcesByClusterAndComponent(db *gorm.DB, clusterID string, componentID uuid.UUID) ([]*MeshSyncResource, error) {
    var resources []*MeshSyncResource
    // This uses the composite index idx_registry_refs for optimal performance
    err := db.Where("cluster_id = ? AND component_id = ?", clusterID, componentID).
        Preload("KubernetesResourceMeta").
        Preload("Spec").
        Preload("Status").
        Find(&resources).Error
    return resources, err
}

// CountResourcesByCluster counts resources in a cluster
func CountResourcesByCluster(db *gorm.DB, clusterID string) (int64, error) {
    var count int64
    err := db.Model(&MeshSyncResource{}).
        Where("cluster_id = ?", clusterID).
        Count(&count).Error
    return count, err
}

// CountResourcesByComponent counts resources of a specific component type
func CountResourcesByComponent(db *gorm.DB, componentID uuid.UUID) (int64, error) {
    var count int64
    err := db.Model(&MeshSyncResource{}).
        Where("component_id = ?", componentID).
        Count(&count).Error
    return count, err
}

// GetResourceByID gets a single resource by ID with all relations loaded
func GetResourceByID(db *gorm.DB, resourceID string) (*MeshSyncResource, error) {
    var resource MeshSyncResource
    err := db.
        Preload("KubernetesResourceMeta").
        Preload("Spec").
        Preload("Status").
        First(&resource, "id = ?", resourceID).Error

    if err != nil {
        return nil, err
    }
    return &resource, nil
}
