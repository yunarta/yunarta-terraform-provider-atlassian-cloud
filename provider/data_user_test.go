package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestUserDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"atlassian": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: `
provider "atlassian" {
  endpoint = "https://mobilesolutionworks.atlassian.net"
  username = "yunarta.kartawahyudi@gmail.com"
  token    = "ATATT3xFfGF0ZWC_HnRuVrq3CSXW0yDTrfq0IVnVsFsnQqOBcp0XkOL-fGB20B6IZn2I9Eyd4FDl2NKsY8VswjaToxxnNpwXHrYEPbd4OPPnm2EUSsLFPBSY3_Qzm-fPviEcwtcu_lLuIXg71p4ZUV6yeAgnVetEZgJWMXsw3woeVk55mEk3F0U=ECB20993"
}

data "atlassian_user" "people" {
  email_address ="yunarta.kartawahyudi@gmail.com"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.atlassian_user.people", "id", "557058:32b276cf-1a9f-45ae-b3f5-f850bc24f1b9"),
				),
			},
		},
	})
}
