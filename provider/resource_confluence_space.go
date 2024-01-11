package provider

import "github.com/yunarta/terraform-atlassian-api-client/jira/cloud"

type ConfluenceSpaceResource struct {
	client *cloud.JiraClient
	model  *AtlassianCloudProviderConfig
}
