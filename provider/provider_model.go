package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/yunarta/terraform-api-transport/transport"
	confluence "github.com/yunarta/terraform-atlassian-api-client/confluence/cloud"
	jira "github.com/yunarta/terraform-atlassian-api-client/jira/cloud"
	"github.com/yunarta/terraform-provider-commons/util"
)

type AtlassianCloudProviderConfig struct {
	EndPoint types.String `tfsdk:"endpoint"`
	Username types.String `tfsdk:"username"`
	Token    types.String `tfsdk:"token"`
}

type ConfigurableForJira interface {
	SetConfig(config *AtlassianCloudProviderConfig, client *jira.JiraClient)
}

type ConfigurableForConfluence interface {
	SetConfig(config *AtlassianCloudProviderConfig, client *confluence.ConfluenceClient)
}

func ConfigureJiraResource(receiver ConfigurableForJira, ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	config, ok := request.ProviderData.(*AtlassianCloudProviderConfig)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *AtlassianCloudProviderModel, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}

	receiver.SetConfig(config, jira.NewJiraClient(
		&util.RecordingHttpPayloadTransport{
			Transport: transport.NewHttpPayloadTransport(config.EndPoint.ValueString(),
				transport.BasicAuthentication{
					Username: config.Username.ValueString(),
					Password: config.Token.ValueString(),
				},
			),
		},
	))
}

func ConfigureConfluenceResource(receiver ConfigurableForConfluence, ctx context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	config, ok := request.ProviderData.(*AtlassianCloudProviderConfig)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *AtlassianCloudProviderModel, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}

	receiver.SetConfig(config, confluence.NewConfluenceClient(
		&util.RecordingHttpPayloadTransport{
			Transport: transport.NewHttpPayloadTransport(config.EndPoint.ValueString(),
				transport.BasicAuthentication{
					Username: config.Username.ValueString(),
					Password: config.Token.ValueString(),
				},
			),
		},
	))
}

func ConfigureJiraDataSource(receiver ConfigurableForJira, ctx context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	config, ok := request.ProviderData.(*AtlassianCloudProviderConfig)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *AtlassianCloudProviderModel, got: %T. Please report this issue to the provider developers.", request.ProviderData),
		)
		return
	}

	receiver.SetConfig(config, jira.NewJiraClient(
		&util.RecordingHttpPayloadTransport{
			Transport: transport.NewHttpPayloadTransport(config.EndPoint.ValueString(),
				transport.BasicAuthentication{
					Username: config.Username.ValueString(),
					Password: config.Token.ValueString(),
				},
			),
		},
	))
}
