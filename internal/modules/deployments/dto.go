package deployments

import "github.com/optikklabs/query/internal/modules/deployments/models"

// Public aliases keep the module's wire DTOs discoverable at its package root
// while allowing the service and repository subpackages to share them without
// an import cycle.
type ListResponse = models.ListResponse
type CompareResponse = models.CompareResponse
type TrafficResponse = models.TrafficResponse
type ErrorChangesResponse = models.ErrorChangesResponse
type DimensionDiffResponse = models.DimensionDiffResponse
