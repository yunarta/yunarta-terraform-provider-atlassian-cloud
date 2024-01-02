package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/yunarta/golang-quality-of-life-pack/collections"
	"github.com/yunarta/terraform-atlassian-api-client/jira/cloud"
)

type ProjectRoleResource interface {
	getClient() *cloud.JiraClient
}

type ProjectRoleInterface interface {
	getAssignment(ctx context.Context) (Assignments, diag.Diagnostics)
	getProjectIdOrKey(ctx context.Context) string
}

func CreateProjectRoleAssignments(ctx context.Context, receiver ProjectRoleResource, plan ProjectRoleInterface) (*AssignmentResult, diag.Diagnostics) {
	plannedAssignment, diags := plan.getAssignment(ctx)
	if diags != nil {
		return nil, diags
	}

	plannedAssignmentOrder, diags := plannedAssignment.CreateAssignmentOrder(ctx)
	if diags != nil {
		return nil, diags
	}

	projectIdOrKey := plan.getProjectIdOrKey(ctx)

	updateService := cloud.NewProjectRoleManager(
		receiver.getClient(),
		projectIdOrKey,
	)

	// Read both in state and planned roles to fill in the update service with prepared data
	_, _ = updateService.ReadRoles(plannedAssignmentOrder.Roles)
	// Register all usernames and groupNames in play to prepare the data
	receiver.getClient().ActorLookupService().RegisterUsernames(
		plannedAssignmentOrder.UserNames...,
	)
	receiver.getClient().ActorLookupService().RegisterGroupNames(
		plannedAssignmentOrder.GroupNames...,
	)
	defer updateService.Finalized()

	return ApplyNewAssignmentSet(ctx, receiver.getClient().ActorLookupService(),
		*plannedAssignmentOrder,
		func(user string, requestedRoles []string) error {
			return updateService.UpdateUserRoles(user, requestedRoles)
		},
		func(group string, requestedRoles []string) error {
			return updateService.UpdateUserRoles(group, requestedRoles)
		},
	)
}

func ComputeProjectRoleAssignments(ctx context.Context, receiver ProjectRoleResource, state ProjectRoleInterface) (*AssignmentResult, diag.Diagnostics) {
	assignments, diags := state.getAssignment(ctx)
	if diags != nil {
		return nil, diags
	}

	assignmentOrder, diags := assignments.CreateAssignmentOrder(ctx)
	if diags != nil {
		return nil, diags
	}

	projectIdOrKey := state.getProjectIdOrKey(ctx)

	updateService := cloud.NewProjectRoleManager(
		receiver.getClient(),
		projectIdOrKey,
	)
	updateService.ReadOnly = true

	assignedRoles, err := updateService.ReadRoles(assignmentOrder.Roles)
	if err != nil {
		return nil, []diag.Diagnostic{diag.NewErrorDiagnostic("Failed to read project roles", err.Error())}
	}

	return ComputeAssignment(ctx, assignedRoles, *assignmentOrder)
}

func UpdateProjectRoleAssignments(ctx context.Context, receiver ProjectRoleResource,
	plan ProjectRoleInterface,
	state ProjectRoleInterface,
	forceUpdate bool) (*AssignmentResult, diag.Diagnostics) {

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
	projectIdOrKey := state.getProjectIdOrKey(ctx)

	updateService := cloud.NewProjectRoleManager(
		receiver.getClient(),
		projectIdOrKey,
	)

	// Read both in state and planned roles to fill in the update service with prepared data
	_, _ = updateService.ReadRoles(append(inStateAssignmentOrder.Roles, plannedAssignmentOrder.Roles...))
	// Register all usernames and groupNames in play to prepare the data
	receiver.getClient().ActorLookupService().RegisterUsernames(
		collections.Unique(append(inStateAssignmentOrder.UserNames, plannedAssignmentOrder.UserNames...))...,
	)
	receiver.getClient().ActorLookupService().RegisterGroupNames(
		collections.Unique(append(inStateAssignmentOrder.GroupNames, plannedAssignmentOrder.GroupNames...))...,
	)
	defer updateService.Finalized()

	return UpdateAssignment(ctx, receiver.getClient().ActorLookupService(),
		*inStateAssignmentOrder,
		*plannedAssignmentOrder,
		forceUpdate,
		func(user string, requestedRoles []string) error {
			return updateService.UpdateUserRoles(user, requestedRoles)
		},
		func(group string, requestedRoles []string) error {
			return updateService.UpdateUserRoles(group, requestedRoles)
		},
	)
}

func DeleteProjectRoleAssignments(ctx context.Context, receiver ProjectRoleResource, state ProjectRoleInterface) diag.Diagnostics {
	assignments, diags := state.getAssignment(ctx)
	if diags != nil {
		return diags
	}

	inStateAssignmentOrder, diags := assignments.CreateAssignmentOrder(ctx)
	if diags != nil {
		return diags
	}

	projectIdOrKey := state.getProjectIdOrKey(ctx)

	updateService := cloud.NewProjectRoleManager(
		receiver.getClient(),
		projectIdOrKey,
	)

	// Read both in state and planned roles to fill in the update service with prepared data
	assignedRoles, _ := updateService.ReadRoles(inStateAssignmentOrder.Roles)
	// Register all usernames and groupNames in play to prepare the data
	receiver.getClient().ActorLookupService().RegisterUsernames(
		inStateAssignmentOrder.UserNames...,
	)
	receiver.getClient().ActorLookupService().RegisterGroupNames(
		inStateAssignmentOrder.GroupNames...,
	)
	defer updateService.Finalized()

	return RemoveAssignment(ctx, assignedRoles, inStateAssignmentOrder,
		func(user string, requestedRoles []string) error {
			return updateService.UpdateUserRoles(user, requestedRoles)
		},
		func(group string, requestedRoles []string) error {
			return updateService.UpdateUserRoles(group, requestedRoles)
		})
}
