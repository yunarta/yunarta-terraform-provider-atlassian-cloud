package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	confluenceApi "github.com/yunarta/terraform-atlassian-api-client/confluence"
	"github.com/yunarta/terraform-atlassian-api-client/confluence/cloud"
	"github.com/yunarta/terraform-provider-atlassian-cloud/provider/confluence"
	"github.com/yunarta/terraform-provider-commons/util"
	"regexp"
)

type ConfluenceSpaceResource struct {
	client *cloud.ConfluenceClient
	model  *AtlassianCloudProviderConfig
}

var (
	_ resource.Resource                = &ConfluenceSpaceResource{}
	_ resource.ResourceWithConfigure   = &ConfluenceSpaceResource{}
	_ resource.ResourceWithImportState = &ConfluenceSpaceResource{}
	_ ConfigurableForConfluence        = &ConfluenceSpaceResource{}
	_ SpaceRoleResource                = &ConfluenceSpaceResource{}
)

func NewConfluenceSpaceResource() resource.Resource {
	return &ConfluenceSpaceResource{}
}

func (receiver *ConfluenceSpaceResource) getClient() *cloud.ConfluenceClient {
	return receiver.client
}

func (receiver *ConfluenceSpaceResource) SetConfig(config *AtlassianCloudProviderConfig, client *cloud.ConfluenceClient) {
	receiver.model = config
	receiver.client = client
}

func (receiver *ConfluenceSpaceResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_confluence_space"
}
func (receiver *ConfluenceSpaceResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"retain_on_delete": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"account_id": schema.Int64Attribute{
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
			"description": schema.StringAttribute{
				Optional: true,
			},

			"assignment_version": schema.StringAttribute{
				Optional: true,
			},
			"computed_users":  confluence.ComputedAssignmentSchema,
			"computed_groups": confluence.ComputedAssignmentSchema,
		},
		Blocks: map[string]schema.Block{
			"assignments": confluence.AssignmentSchema(),
		},
	}
}

func (receiver *ConfluenceSpaceResource) Configure(ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	ConfigureConfluenceResource(receiver, ctx, request, response)
}

func (receiver *ConfluenceSpaceResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var (
		diags diag.Diagnostics
		err   error

		plan SpaceModel
	)

	diags = request.Plan.Get(ctx, &plan)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	createSpace, err := receiver.client.SpaceService().Create(confluenceApi.CreateSpace{
		Key:  plan.Key.ValueString(),
		Name: plan.Name.ValueString(),
		Description: confluenceApi.Description{
			Plain: confluenceApi.ContentValue{
				Value: plan.Description.ValueString(),
			},
		},
	})
	if util.TestError(&response.Diagnostics, err, "failed to create project") {
		return
	}

	plan.AccountId = types.Int64Value(createSpace.Id)

	diags = response.State.Set(ctx, plan)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	computation, diags := CreateSpaceRoleAssignments(ctx, receiver, plan)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	spaceModel := NewSpaceModel(plan, createSpace, computation)

	diags = response.State.Set(ctx, spaceModel)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}

func (receiver *ConfluenceSpaceResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var (
		diags diag.Diagnostics
		err   error

		state SpaceModel
		space *confluenceApi.Space
	)

	diags = request.State.Get(ctx, &state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	space, err = receiver.client.SpaceService().Read(state.Key.ValueString())
	if util.TestError(&response.Diagnostics, err, "failed to remove project") {
		return
	}

	computation, diags := ComputeSpaceRoleAssignments(ctx, receiver, state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	spaceModel := NewSpaceModel(state, space, computation)

	diags = response.State.Set(ctx, &spaceModel)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}

func (receiver *ConfluenceSpaceResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var (
		diags diag.Diagnostics
		err   error

		plan, state SpaceModel
		computation *confluence.AssignmentResult
	)

	diags = request.Plan.Get(ctx, &plan)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	diags = request.State.Get(ctx, &state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	space, err := receiver.client.SpaceService().Update(state.Key.ValueString(), confluenceApi.UpdateSpace{
		Name: plan.Name.ValueString(),
		Description: confluenceApi.Description{
			Plain: confluenceApi.ContentValue{
				Value: plan.Description.ValueString(),
			},
		},
	})

	if util.TestError(&response.Diagnostics, err, "Failed to update deployment") {
		return
	}

	forceUpdate := !plan.AssignmentVersion.Equal(state.AssignmentVersion)
	computation, diags = UpdateSpaceRoleAssignments(ctx, receiver, plan, state, forceUpdate)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	spaceModel := NewSpaceModel(plan, space, computation)

	diags = response.State.Set(ctx, spaceModel)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}

func (receiver *ConfluenceSpaceResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var (
		diags diag.Diagnostics
		err   error

		state SpaceModel
	)

	diags = request.State.Get(ctx, &state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	if !state.RetainOnDelete.ValueBool() {
		err = receiver.client.SpaceService().Delete(state.Key.ValueString())
		if util.TestError(&response.Diagnostics, err, "failed to remove project") {
			return
		}
	}

	response.State.RemoveResource(ctx)
}

func (receiver *ConfluenceSpaceResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	diags := response.State.SetAttribute(ctx, path.Root("key"), request.ID)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}
