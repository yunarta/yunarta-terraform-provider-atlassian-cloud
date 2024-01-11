package provider

import "github.com/yunarta/terraform-atlassian-api-client/jira/cloud"

// BoardResource is used to create board that contains multiple projects
type BoardResource struct {
	client *cloud.JiraClient
	model  *AtlassianCloudProviderConfig
}
