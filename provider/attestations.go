package provider

import (
	"github.com/yunarta/terraform-atlassian-api-client/jira"
	"sort"
)

func CreateAttestation(roles *jira.ObjectRoles) (map[string][]string, map[string][]string) {
	var userRolesMap = make(map[string][]string)
	var groupRolesMap = make(map[string][]string)
	for _, user := range roles.Users {
		for _, role := range user.Roles {
			if role == "atlassian-addons-project-access" {
				continue
			}

			userInRole, ok := userRolesMap[role]
			if !ok {
				userInRole = make([]string, 0)
				userRolesMap[role] = userInRole
			}

			userInRole = append(userInRole, user.Name)
			userRolesMap[role] = userInRole
		}
	}

	for _, group := range roles.Groups {
		for _, role := range group.Roles {
			if role == "atlassian-addons-project-access" {
				continue
			}

			groupInRole, ok := groupRolesMap[role]
			if !ok {
				groupInRole = make([]string, 0)
			}

			groupInRole = append(groupInRole, group.Name)
			groupRolesMap[role] = groupInRole
		}
	}

	for _, groups := range groupRolesMap {
		sort.Strings(groups)
	}

	for _, users := range userRolesMap {
		sort.Strings(users)
	}

	return userRolesMap, groupRolesMap
}
