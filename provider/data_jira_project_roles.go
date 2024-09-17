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

type JiraProjectRolesData struct {
	Key    string              `tfsdk:"key"`
	Users  map[string][]string `tfsdk:"users"`
	Groups map[string][]string `tfsdk:"groups"`
}

type JiraProjectRolesDataSource struct {
	client *cloud.JiraClient
	model  *AtlassianCloudProviderConfig
}

var (
	_ datasource.DataSource              = &JiraProjectRolesDataSource{}
	_ datasource.DataSourceWithConfigure = &JiraProjectRolesDataSource{}
	_ ConfigurableForJira                = &JiraProjectRolesDataSource{}
)

func NewJiraProjectRolesDataSource() datasource.DataSource {
	return &JiraProjectRolesDataSource{}
}

func (receiver *JiraProjectRolesDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_jira_project_roles"
}

func (receiver *JiraProjectRolesDataSource) SetConfig(config *AtlassianCloudProviderConfig, client *cloud.JiraClient) {
	receiver.model = config
	receiver.client = client
}

func (receiver *JiraProjectRolesDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				Required: true,
			},
			"users": schema.MapAttribute{
				Computed: true,
				ElementType: types.ListType{
					ElemType: types.StringType,
				},
			},
			"groups": schema.MapAttribute{
				Computed: true,
				ElementType: types.ListType{
					ElemType: types.StringType,
				},
			},
		},
	}
}

func (receiver *JiraProjectRolesDataSource) Configure(ctx context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	ConfigureJiraDataSource(receiver, ctx, request, response)
}

func (receiver *JiraProjectRolesDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var (
		diags diag.Diagnostics
		err   error

		state JiraProjectRolesData
	)

	response.Diagnostics = make(diag.Diagnostics, 0)

	diags = request.Config.Get(ctx, &state)
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}

	jiraProjectRoles, err := receiver.client.ProjectRoleService().ReadProjectRoles(state.Key)
	if util.TestError(&response.Diagnostics, err, "failed to find project") {
		return
	}

	if jiraProjectRoles == nil {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("Failed to find project", state.Key))
		return
	}

	manager := cloud.NewProjectRoleManager(receiver.client, state.Key)
	_, err = manager.ReadAllRoles()
	objectRoles, err := manager.ReadAllRoles()

	users, groups := CreateAttestation(objectRoles)
	diags = response.State.Set(ctx, &JiraProjectRolesData{
		Key:    state.Key,
		Users:  users,
		Groups: groups,
	})
	if util.TestDiagnostic(&response.Diagnostics, diags) {
		return
	}
}
