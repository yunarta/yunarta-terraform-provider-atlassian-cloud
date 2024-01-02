package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/yunarta/terraform-atlassian-api-client/jira"
	"github.com/yunarta/terraform-atlassian-api-client/jira/cloud"
	"github.com/yunarta/terraform-provider-commons/util"
	"regexp"
)

type ProjectResource struct {
	client *cloud.JiraClient
	model  *AtlassianCloudProviderConfig
}

var (
	_ resource.Resource                = &ProjectResource{}
	_ resource.ResourceWithConfigure   = &ProjectResource{}
	_ resource.ResourceWithImportState = &ProjectResource{}
	_ Configurable                     = &ProjectResource{}
	_ ProjectRoleResource              = &ProjectResource{}
)

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

func (receiver *ProjectResource) getClient() *cloud.JiraClient {
	return receiver.client
}

func (receiver *ProjectResource) SetConfig(config *AtlassianCloudProviderConfig, client *cloud.JiraClient) {
	receiver.model = config
	receiver.client = client
}

func (receiver *ProjectResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_jira_project"
}
func (receiver *ProjectResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"account_id": schema.StringAttribute{
				Computed: true,
			},
			"key": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`),
						"value must be a numeric",
					),
					stringvalidator.LengthAtMost(10),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"project_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("business", "service_desk", "software"),
				},
			},
			"description": schema.StringAttribute{
				Optional: true,
			},
			"category_id": schema.Int64Attribute{
				Optional: true,
			},
			"lead_account": schema.StringAttribute{
				Required: true,
			},
			"default_assignee": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("PROJECT_LEAD", "UNASSIGNED"),
				},
			},
			"delete_to_trash": schema.BoolAttribute{
				Optional: true,
			},

			"assignment_version": schema.StringAttribute{
				Optional: true,
			},
			"computed_users":  ComputedAssignmentSchema,
			"computed_groups": ComputedAssignmentSchema,
		},
		Blocks: map[string]schema.Block{
			"assignments": AssignmentSchema(),
		},
	}
}

func (receiver *ProjectResource) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	ConfigureResource(receiver, ctx, request, response)
}

func (receiver *ProjectResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var (
		diags diag.Diagnostics
		err   error

		plan ProjectModel
	)

	diags = request.Plan.Get(ctx, &plan)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	project := jira.CreateProject{
		Key:            plan.Key.ValueString(),
		Name:           plan.Name.ValueString(),
		ProjectTypeKey: plan.ProjectType.ValueString(),
		Description:    plan.Description.ValueString(),
		CategoryId:     int(plan.CategoryId.ValueInt64()),
		LeadAccountId:  plan.LeadAccount.ValueString(),
		AssigneeType:   plan.DefaultAssignee.ValueString(),
	}

	createdProject, err := receiver.client.ProjectService().Create(project)
	if util.TestError(&response.Diagnostics, err, "failed to create project") {
		return
	}

	plan.AccountId = types.StringValue(createdProject.ID)

	diags = response.State.Set(ctx, plan)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	computation, diags := CreateProjectRoleAssignments(ctx, receiver, plan)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	deploymentModel := NewProjectModel(createdProject, plan, computation)

	diags = response.State.Set(ctx, deploymentModel)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}

func (receiver *ProjectResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var (
		diags diag.Diagnostics
		err   error

		state   ProjectModel
		project *jira.Project
	)

	diags = request.State.Get(ctx, &state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	project, err = receiver.client.ProjectService().Read(state.Key.ValueString())
	if util.TestError(&response.Diagnostics, err, "failed to remove project") {
		return
	}

	computation, diags := ComputeProjectRoleAssignments(ctx, receiver, state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	projectModel := NewProjectModel(project, state, computation)

	diags = response.State.Set(ctx, &projectModel)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}

func (receiver *ProjectResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var (
		diags diag.Diagnostics
		err   error

		plan, state ProjectModel
		computation *AssignmentResult
	)

	diags = request.Plan.Get(ctx, &plan)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	diags = request.State.Get(ctx, &state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	var categoryId = -1
	if !plan.CategoryId.IsNull() {
		categoryId = int(plan.CategoryId.ValueInt64())
	}

	project, err := receiver.client.ProjectService().Update(state.Key.ValueString(), jira.UpdateProject{
		Name:          plan.Name.ValueString(),
		LeadAccountId: plan.LeadAccount.ValueString(),
		Description:   plan.Description.ValueString(),
		AssigneeType:  plan.DefaultAssignee.ValueString(),
		CategoryId:    categoryId,
	})

	if util.TestError(&response.Diagnostics, err, "Failed to update deployment") {
		return
	}

	forceUpdate := !plan.AssignmentVersion.Equal(state.AssignmentVersion)
	computation, diags = UpdateProjectRoleAssignments(ctx, receiver, plan, state, forceUpdate)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	deploymentModel := NewProjectModel(project, plan, computation)

	diags = response.State.Set(ctx, deploymentModel)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}

func (receiver *ProjectResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var (
		diags diag.Diagnostics
		err   error

		state ProjectModel
	)

	diags = request.State.Get(ctx, &state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	_, err = receiver.client.ProjectService().Delete(state.Key.ValueString(), state.DeleteToTrash.ValueBool())
	if util.TestError(&response.Diagnostics, err, "failed to remove project") {
		return
	}

	response.State.RemoveResource(ctx)
}

func (receiver *ProjectResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	diags := response.State.SetAttribute(ctx, path.Root("key"), request.ID)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}
