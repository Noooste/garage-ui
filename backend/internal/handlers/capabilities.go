package handlers

import (
	"sort"

	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

type CapabilitiesHandler struct {
	apiVersion           string
	capabilities         services.Capabilities
	accessControlEnabled bool
}

func NewCapabilitiesHandler(apiVersion string, capabilities services.Capabilities, accessControlEnabled bool) *CapabilitiesHandler {
	return &CapabilitiesHandler{
		apiVersion:           apiVersion,
		capabilities:         capabilities,
		accessControlEnabled: accessControlEnabled,
	}
}

// accessControlBinding mirrors one compiled binding, unflattened: "read on
// backend-*" plus "write on data-*" must never merge into both-on-both.
type accessControlBinding struct {
	BucketPrefixes []string `json:"bucket_prefixes"`
	Permissions    []string `json:"permissions"`
}

type accessControlBlock struct {
	Enabled            bool                   `json:"enabled"`
	Subject            string                 `json:"subject,omitempty"`
	IsAdmin            bool                   `json:"is_admin,omitempty"`
	Bindings           []accessControlBinding `json:"bindings,omitempty"`
	ClusterPermissions []string               `json:"cluster_permissions,omitempty"`
}

func (h *CapabilitiesHandler) GetCapabilities(c fiber.Ctx) error {
	ac := accessControlBlock{Enabled: h.accessControlEnabled}
	if h.accessControlEnabled {
		if subj, ok := authz.SubjectFrom(c); ok {
			ac.Subject = subj.ID
			ac.IsAdmin = subj.IsAdmin
			for _, b := range subj.Bindings {
				ac.Bindings = append(ac.Bindings, accessControlBinding{
					BucketPrefixes: b.BucketPrefixes,
					Permissions:    sortedPerms(b.Permissions),
				})
			}
			ac.ClusterPermissions = sortedPerms(subj.ClusterPerms)
		}
	}
	return c.JSON(models.SuccessResponse(fiber.Map{
		"garageApiVersion": h.apiVersion,
		"features":         h.capabilities,
		"access_control":   ac,
	}))
}

func sortedPerms(set authz.PermSet) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
