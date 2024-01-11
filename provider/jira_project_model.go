package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/yunarta/terraform-atlassian-api-client/jira"
	"github.com/yunarta/terraform-provider-commons/util"
	"strconv"
)

type ProjectModel struct {
	RetainOnDelete  types.Bool   `tfsdk:"retain_on_delete"`
	AccountId       types.String `tfsdk:"account_id"`
	Key             types.String `tfsdk:"key"`
	Name            types.String `tfsdk:"name"`
	ProjectType     types.String `tfsdk:"project_type"`
	Description     types.String `tfsdk:"description"`
	CategoryId      types.Int64  `tfsdk:"category_id"`
	LeadAccount     types.String `tfsdk:"lead_account"`
	DefaultAssignee types.String `tfsdk:"default_assignee"`
	DeleteToTrash   types.Bool   `tfsdk:"delete_to_trash"`

	AssignmentVersion types.String `tfsdk:"assignment_version"`
	Assignments       types.List   `tfsdk:"assignments"`
	ComputedUsers     types.List   `tfsdk:"computed_users"`
	ComputedGroups    types.List   `tfsdk:"computed_groups"`
}

var _ ProjectRoleInterface = &ProjectModel{}

func (p ProjectModel) getAssignment(ctx context.Context) (Assignments, diag.Diagnostics) {
	var assignments Assignments = make([]Assignment, 0)

	diags := p.Assignments.ElementsAs(ctx, &assignments, true)
	return assignments, diags
}

func (p ProjectModel) getProjectIdOrKey(ctx context.Context) string {
	return p.Key.ValueString()
}

func NewProjectModel(plan ProjectModel, project *jira.Project, assignmentResult *AssignmentResult) *ProjectModel {
	var categoryId types.Int64
	if len(project.ProjectCategory.ID) > 0 {
		value, _ := strconv.Atoi(project.ProjectCategory.ID)
		categoryId = types.Int64Value(int64(value))
	} else {
		categoryId = types.Int64Null()
	}

	return &ProjectModel{
		RetainOnDelete:    plan.RetainOnDelete,
		AccountId:         types.StringValue(project.ID),
		Key:               types.StringValue(project.Key),
		Name:              types.StringValue(project.Name),
		ProjectType:       types.StringValue(project.ProjectTypeKey),
		Description:       util.NullString(project.Description),
		CategoryId:        categoryId,
		LeadAccount:       types.StringValue(project.Lead.AccountID),
		DefaultAssignee:   types.StringValue(project.AssigneeType),
		AssignmentVersion: plan.AssignmentVersion,
		Assignments:       plan.Assignments,
		ComputedUsers:     assignmentResult.ComputedUsers,
		ComputedGroups:    assignmentResult.ComputedGroups,
	}
}
