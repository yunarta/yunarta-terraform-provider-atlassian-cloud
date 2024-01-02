package test

import (
	"github.com/stretchr/testify/assert"
	"github.com/yunarta/terraform-atlassian-api-client/jira/cloud"
	"testing"
)

func TestUserSearchHandler(t *testing.T) {
	var err error

	var client = cloud.NewJiraClient(NewJiraTransport())

	success, err := client.ActorService().ReadUser("yunarta.kartawahyudi@gmail.com")
	assert.Nil(t, err)
	assert.NotNil(t, success)
}
