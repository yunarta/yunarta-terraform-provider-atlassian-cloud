package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/yunarta/golang-quality-of-life-pack/collections"
	"github.com/yunarta/terraform-atlassian-api-client/confluence/cloud"
	"github.com/yunarta/terraform-provider-atlassian-cloud/provider/confluence"
)

type SpaceRoleResource interface {
	getClient() *cloud.ConfluenceClient
}

type SpaceRoleInterface interface {
	getAssignment(ctx context.Context) (confluence.Assignments, diag.Diagnostics)
	getSpaceIdOrKey(ctx context.Context) string
}

func CreateSpaceRoleAssignments(ctx context.Context, receiver SpaceRoleResource, plan SpaceRoleInterface) (*confluence.AssignmentResult, diag.Diagnostics) {
	plannedAssignment, diags := plan.getAssignment(ctx)
	if diags != nil {
		return nil, diags
	}

	plannedAssignmentOrder, diags := plannedAssignment.CreateAssignmentOrder(ctx)
	if diags != nil {
		return nil, diags
	}

	SpaceIdOrKey := plan.getSpaceIdOrKey(ctx)

	updateService := cloud.NewSpaceRoleManager(
		receiver.getClient(),
		SpaceIdOrKey,
	)

	// Read both in state and planned roles to fill in the update service with prepared data
	_, _ = updateService.ReadPermissions()
	// Register all usernames and groupNames in play to prepare the data
	receiver.getClient().ActorLookupService().RegisterUsernames(
		plannedAssignmentOrder.UserNames...,
	)
	receiver.getClient().ActorLookupService().RegisterGroupNames(
		plannedAssignmentOrder.GroupNames...,
	)

	return confluence.ApplyNewAssignmentSet(ctx, receiver.getClient().ActorLookupService(),
		*plannedAssignmentOrder,
		func(user string, requestedRoles []string) error {
			return updateService.UpdateUserPermissions(user, requestedRoles)
		},
		func(group string, requestedRoles []string) error {
			return updateService.UpdateUserPermissions(group, requestedRoles)
		},
	)
}

func ComputeSpaceRoleAssignments(ctx context.Context, receiver SpaceRoleResource, state SpaceRoleInterface) (*confluence.AssignmentResult, diag.Diagnostics) {
	assignments, diags := state.getAssignment(ctx)
	if diags != nil {
		return nil, diags
	}

	assignmentOrder, diags := assignments.CreateAssignmentOrder(ctx)
	if diags != nil {
		return nil, diags
	}

	SpaceIdOrKey := state.getSpaceIdOrKey(ctx)

	updateService := cloud.NewSpaceRoleManager(
		receiver.getClient(),
		SpaceIdOrKey,
	)

	assignedRoles, err := updateService.ReadPermissions()
	if err != nil {
		return nil, []diag.Diagnostic{diag.NewErrorDiagnostic("Failed to read Space roles", err.Error())}
	}

	return confluence.ComputePermissionAssignments(ctx, assignedRoles, *assignmentOrder)
}

func UpdateSpaceRoleAssignments(ctx context.Context, receiver SpaceRoleResource,
	plan SpaceRoleInterface,
	state SpaceRoleInterface,
	forceUpdate bool) (*confluence.AssignmentResult, diag.Diagnostics) {

	plannedAssignments, diags := plan.getAssignment(ctx)
	if diags != nil {
		return nil, diags
	}

	inStateAssignments, diags := state.getAssignment(ctx)
	if diags != nil {
		return nil, diags
	}

	plannedAssignmentOrder, diags := plannedAssignments.CreateAssignmentOrder(ctx)
	if diags != nil {
		return nil, diags
	}

	inStateAssignmentOrder, diags := inStateAssignments.CreateAssignmentOrder(ctx)
	if diags != nil {
		return nil, diags
	}

	// the plan does not have computed value deployment ID
	SpaceIdOrKey := state.getSpaceIdOrKey(ctx)

	updateService := cloud.NewSpaceRoleManager(
		receiver.getClient(),
		SpaceIdOrKey,
	)

	// Read both in state and planned roles to fill in the update service with prepared data
	_, _ = updateService.ReadPermissions()
	// Register all usernames and groupNames in play to prepare the data
	receiver.getClient().ActorLookupService().RegisterUsernames(
		collections.Unique(append(inStateAssignmentOrder.UserNames, plannedAssignmentOrder.UserNames...))...,
	)
	receiver.getClient().ActorLookupService().RegisterGroupNames(
		collections.Unique(append(inStateAssignmentOrder.GroupNames, plannedAssignmentOrder.GroupNames...))...,
	)
	//defer updateService.Finalized()

	return confluence.UpdateAssignment(ctx, receiver.getClient().ActorLookupService(),
		*inStateAssignmentOrder,
		*plannedAssignmentOrder,
		forceUpdate,
		func(user string, requestedRoles []string) error {
			return updateService.UpdateUserPermissions(user, requestedRoles)
		},
		func(group string, requestedRoles []string) error {
			return updateService.UpdateUserPermissions(group, requestedRoles)
		},
	)
}

func DeleteSpaceRoleAssignments(ctx context.Context, receiver SpaceRoleResource, state SpaceRoleInterface) diag.Diagnostics {
	assignments, diags := state.getAssignment(ctx)
	if diags != nil {
		return diags
	}

	inStateAssignmentOrder, diags := assignments.CreateAssignmentOrder(ctx)
	if diags != nil {
		return diags
	}

	SpaceIdOrKey := state.getSpaceIdOrKey(ctx)

	updateService := cloud.NewSpaceRoleManager(
		receiver.getClient(),
		SpaceIdOrKey,
	)

	// Read both in state and planned roles to fill in the update service with prepared data
	assignedRoles, _ := updateService.ReadPermissions()
	// Register all usernames and groupNames in play to prepare the data
	receiver.getClient().ActorLookupService().RegisterUsernames(
		inStateAssignmentOrder.UserNames...,
	)
	receiver.getClient().ActorLookupService().RegisterGroupNames(
		inStateAssignmentOrder.GroupNames...,
	)
	//defer updateService.Finalized()

	return confluence.RemoveAssignment(ctx, assignedRoles, inStateAssignmentOrder,
		func(user string, requestedRoles []string) error {
			return updateService.UpdateUserPermissions(user, requestedRoles)
		},
		func(group string, requestedRoles []string) error {
			return updateService.UpdateUserPermissions(group, requestedRoles)
		})
}
