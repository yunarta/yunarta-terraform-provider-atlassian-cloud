package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/yunarta/terraform-atlassian-api-client/jira/cloud"
	"github.com/yunarta/terraform-provider-commons/util"
)

type UserData struct {
	Id           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	EmailAddress types.String `tfsdk:"email_address"`
}

type UserDataSource struct {
	client *cloud.JiraClient
	model  *AtlassianCloudProviderConfig
}

var (
	_ datasource.DataSource              = &UserDataSource{}
	_ datasource.DataSourceWithConfigure = &UserDataSource{}
	_ ConfigurableForJira                = &UserDataSource{}
)

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

func (receiver *UserDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_user"
}

func (receiver *UserDataSource) SetConfig(config *AtlassianCloudProviderConfig, client *cloud.JiraClient) {
	receiver.model = config
	receiver.client = client
}

func (receiver *UserDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"email_address": schema.StringAttribute{
				Required: true,
			},
		},
	}
}

func (receiver *UserDataSource) Configure(ctx context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	ConfigureJiraDataSource(receiver, ctx, request, response)
}

func (receiver *UserDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var (
		diags diag.Diagnostics
		err   error

		state UserData
	)

	response.Diagnostics = make(diag.Diagnostics, 0)

	diags = request.Config.Get(ctx, &state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	user, err := receiver.client.ActorService().ReadUser(state.EmailAddress.ValueString())
	if util.TestError(&response.Diagnostics, err, "failed to find user project") {
		return
	}

	if user == nil {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("Failed to find user", state.EmailAddress.ValueString()))
		return
	}

	// crashed here
	diags = response.State.Set(ctx, &UserData{
		Id:           types.StringValue(user.AccountID),
		Name:         types.StringValue(user.Name),
		EmailAddress: types.StringValue(user.EmailAddress),
	})
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}
