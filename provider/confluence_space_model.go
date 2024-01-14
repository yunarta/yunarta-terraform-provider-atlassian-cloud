package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	clientApi "github.com/yunarta/terraform-atlassian-api-client/confluence"
	"github.com/yunarta/terraform-provider-atlassian-cloud/provider/confluence"
	"github.com/yunarta/terraform-provider-commons/util"
)

type SpaceModel struct {
	RetainOnDelete types.Bool   `tfsdk:"retain_on_delete"`
	AccountId      types.Int64  `tfsdk:"account_id"`
	Key            types.String `tfsdk:"key"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`

	AssignmentVersion types.String `tfsdk:"assignment_version"`
	Assignments       types.List   `tfsdk:"assignments"`
	ComputedUsers     types.List   `tfsdk:"computed_users"`
	ComputedGroups    types.List   `tfsdk:"computed_groups"`
}

var _ SpaceRoleInterface = &SpaceModel{}

func (s SpaceModel) getAssignment(ctx context.Context) (confluence.Assignments, diag.Diagnostics) {
	var assignments confluence.Assignments = make([]confluence.Assignment, 0)

	diags := s.Assignments.ElementsAs(ctx, &assignments, true)
	return assignments, diags
}

func (s SpaceModel) getSpaceIdOrKey(ctx context.Context) string {
	return s.Key.ValueString()
}

func NewSpaceModel(plan SpaceModel, project *clientApi.Space, assignmentResult *confluence.AssignmentResult) *SpaceModel {
	return &SpaceModel{
		RetainOnDelete:    plan.RetainOnDelete,
		AccountId:         types.Int64Value(project.Id),
		Key:               types.StringValue(project.Key),
		Name:              types.StringValue(project.Name),
		Description:       util.NullString(project.Description.Plain.Value),
		AssignmentVersion: plan.AssignmentVersion,
		Assignments:       plan.Assignments,
		ComputedUsers:     assignmentResult.ComputedUsers,
		ComputedGroups:    assignmentResult.ComputedGroups,
	}
}
