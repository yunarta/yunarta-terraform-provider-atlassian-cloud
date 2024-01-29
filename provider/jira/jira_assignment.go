package jira

import (
	"context"
	"github.com/emirpasic/gods/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/yunarta/golang-quality-of-life-pack/collections"
	"github.com/yunarta/terraform-atlassian-api-client/jira"
	"github.com/yunarta/terraform-atlassian-api-client/jira/cloud"
	"slices"
	"sort"
	"strings"
)

type Assignment struct {
	Users    []string `tfsdk:"users"`
	Groups   []string `tfsdk:"groups"`
	Roles    []string `tfsdk:"roles"`
	Priority int64    `tfsdk:"priority"`
}

type AssignmentOrder struct {
	Roles      []string
	Users      map[string][]string
	UserNames  []string
	Groups     map[string][]string
	GroupNames []string
}

type Assignments []Assignment

type UpdateUserRolesFunc func(user string, requestedRoles []string) error
type UpdateGroupRolesFunc func(group string, requestedRoles []string) error

func (assignments Assignments) CreateAssignmentOrder(ctx context.Context) (*AssignmentOrder, diag.Diagnostics) {
	var priorities []int64
	var makeAssignments = map[int64]Assignment{}
	for _, assignment := range assignments {
		priorities = append(priorities, assignment.Priority)
		makeAssignments[assignment.Priority] = assignment
	}
	slices.SortFunc(priorities, func(a, b int64) int {
		return utils.Int64Comparator(a, b)
	})
	var usersAssignments = map[string][]string{}
	var groupsAssignments = map[string][]string{}
	var userNames = make([]string, 0)
	var groupNames = make([]string, 0)
	var roles = make([]string, 0)
	for _, priority := range priorities {
		assignment := makeAssignments[priority]
		for _, user := range assignment.Users {
			usersAssignments[user] = assignment.Roles
			userNames = append(userNames, user)
			roles = append(roles, assignment.Roles...)
		}

		for _, group := range assignment.Groups {
			groupsAssignments[group] = assignment.Roles
			groupNames = append(groupNames, group)
			roles = append(roles, assignment.Roles...)
		}
	}

	return &AssignmentOrder{
		Roles:      collections.Unique(roles),
		Users:      usersAssignments,
		UserNames:  userNames,
		Groups:     groupsAssignments,
		GroupNames: groupNames,
	}, nil
}

func AssignmentSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"users": schema.ListAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
				"groups": schema.ListAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
				"roles": schema.ListAttribute{
					Required:    true,
					ElementType: types.StringType,
				},
				"priority": schema.Int64Attribute{
					Required: true,
				},
			},
		},
	}
}

var ComputedAssignmentSchema = schema.ListNestedAttribute{
	Computed: true,
	NestedObject: schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Computed: true,
			},
			"roles": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	},
}

type ComputedAssignment struct {
	Name  string   `tfsdk:"name"`
	Roles []string `tfsdk:"roles"`
}

var assignmentType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"roles": types.ListType{
			ElemType: types.StringType,
		},
		"priority": types.NumberType,
		"users": types.ListType{
			ElemType: types.StringType,
		},
		"groups": types.ListType{
			ElemType: types.StringType,
		},
	},
}

var computedAssignmentType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"roles": types.ListType{ElemType: types.StringType},
		"name":  types.StringType,
	},
}

type AssignmentResult struct {
	ComputedUsers  types.List
	ComputedGroups types.List
}

func ApplyNewAssignmentSet(ctx context.Context, actorLookupService *cloud.ActorLookupService,
	assignmentOrder AssignmentOrder,
	updateUserRoles UpdateUserRolesFunc,
	updateGroupRoles UpdateGroupRolesFunc) (*AssignmentResult, diag.Diagnostics) {

	var err error

	computedUsers := make([]ComputedAssignment, 0)
	computedGroups := make([]ComputedAssignment, 0)

	for user, requestedRoles := range assignmentOrder.Users {
		found := actorLookupService.FindUser(user)
		if found == nil {
			continue
		}

		computedUsers = append(computedUsers, ComputedAssignment{
			Name:  user,
			Roles: requestedRoles,
		})

		err = updateUserRoles(user, requestedRoles)
		if err != nil {
			return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToUpdateUserRoles, err.Error())}
		}
	}

	for group, requestedRoles := range assignmentOrder.Groups {
		found := actorLookupService.FindGroup(group)
		if found == nil {
			continue
		}

		computedGroups = append(computedGroups, ComputedAssignment{
			Name:  group,
			Roles: requestedRoles,
		})

		err = updateGroupRoles(group, requestedRoles)
		if err != nil {
			return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToUpdateGroupRoles, err.Error())}
		}
	}

	sort.Slice(computedUsers, func(a, b int) bool {
		return computedUsers[a].Name > computedUsers[b].Name
	})
	sort.Slice(computedGroups, func(a, b int) bool {
		return computedGroups[a].Name > computedGroups[b].Name
	})

	return createAssignmentResult(ctx, computedUsers, computedGroups)
}

func UpdateAssignment(ctx context.Context, actorLookupService *cloud.ActorLookupService,
	inStateAssignmentOrder AssignmentOrder,
	plannedAssignmentOrder AssignmentOrder,
	forceUpdate bool,
	updateUserRole UpdateUserRolesFunc,
	updateGroupRole UpdateGroupRolesFunc) (*AssignmentResult, diag.Diagnostics) {

	computedUsers, diags := updateUsers(inStateAssignmentOrder, plannedAssignmentOrder, actorLookupService, forceUpdate, updateUserRole)
	if diags != nil {
		return nil, diags
	}

	computedGroups, diags := updateGroups(inStateAssignmentOrder, plannedAssignmentOrder, actorLookupService, forceUpdate, updateGroupRole)
	if diags != nil {
		return nil, diags
	}

	return createAssignmentResult(ctx, computedUsers, computedGroups)
}

func updateUsers(inStateAssignmentOrder AssignmentOrder, plannedAssignmentOrder AssignmentOrder,
	actorLookupService *cloud.ActorLookupService, forceUpdate bool, updateUserRoles UpdateUserRolesFunc) ([]ComputedAssignment, diag.Diagnostics) {
	var err error

	var computedUsers = make([]ComputedAssignment, 0)
	_, removing := collections.Delta(inStateAssignmentOrder.UserNames, plannedAssignmentOrder.UserNames)
	for _, user := range plannedAssignmentOrder.UserNames {
		if collections.Contains(removing, user) {
			continue
		}

		found := actorLookupService.FindUser(user)
		if found == nil {
			continue
		}

		requestedRoles := plannedAssignmentOrder.Users[user]
		inStateRoles := inStateAssignmentOrder.Users[user]
		computedUsers = append(computedUsers, ComputedAssignment{
			Name:  user,
			Roles: requestedRoles,
		})

		if !collections.EqualsIgnoreOrder(inStateRoles, requestedRoles) || forceUpdate {
			err = updateUserRoles(user, requestedRoles)
			if err != nil {
				return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToUpdateUserRoles, err.Error())}
			}
		}
	}

	for _, user := range removing {
		err := updateUserRoles(user, make([]string, 0))
		if err != nil {
			return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToRemoveUserRoles, err.Error())}
		}
	}
	return computedUsers, nil
}

func updateGroups(inStateAssignmentOrder AssignmentOrder, plannedAssignmentOrder AssignmentOrder,
	actorLookupService *cloud.ActorLookupService, forceUpdate bool, updateGroupRoles UpdateGroupRolesFunc) ([]ComputedAssignment, diag.Diagnostics) {
	var err error
	var computedGroups = make([]ComputedAssignment, 0)

	_, removing := collections.Delta(inStateAssignmentOrder.GroupNames, plannedAssignmentOrder.GroupNames)
	for _, group := range plannedAssignmentOrder.GroupNames {
		if collections.Contains(removing, group) {
			continue
		}

		found := actorLookupService.FindGroup(group)
		if found == nil {
			continue
		}

		requestedRoles := plannedAssignmentOrder.Groups[group]
		inStateRoles := inStateAssignmentOrder.Groups[group]
		computedGroups = append(computedGroups, ComputedAssignment{
			Name:  group,
			Roles: requestedRoles,
		})

		if !collections.EqualsIgnoreOrder(inStateRoles, requestedRoles) || forceUpdate {
			err = updateGroupRoles(group, requestedRoles)
			if err != nil {
				return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToUpdateGroupRoles, err.Error())}
			}
		}
	}

	for _, group := range removing {
		err := updateGroupRoles(group, make([]string, 0))
		if err != nil {
			return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToRemoveGroupRoles, err.Error())}
		}
	}

	return computedGroups, nil
}

func RemoveAssignment(ctx context.Context,
	assignedRoles *jira.ObjectRoles, assignmentOrder *AssignmentOrder,
	updateUserRoles UpdateUserRolesFunc,
	updateGroupRoles UpdateGroupRolesFunc) diag.Diagnostics {

	for _, user := range assignedRoles.Users {
		if _, ok := assignmentOrder.Users[user.Name]; ok {
			err := updateUserRoles(user.Name, make([]string, 0))
			if err != nil {
				return []diag.Diagnostic{diag.NewErrorDiagnostic(failedToRemoveUserRoles, err.Error())}
			}
		}
	}

	for _, group := range assignedRoles.Groups {
		if _, ok := assignmentOrder.Groups[group.Name]; ok {
			err := updateGroupRoles(group.Name, make([]string, 0))
			if err != nil {
				return []diag.Diagnostic{diag.NewErrorDiagnostic(failedToRemoveGroupRoles, err.Error())}
			}
		}
	}

	return nil
}

func ComputeJiraAssignment(ctx context.Context,
	assignedRoles *jira.ObjectRoles, assignmentOrder AssignmentOrder) (*AssignmentResult, diag.Diagnostics) {

	computedUsers := make([]ComputedAssignment, 0)
	computedGroups := make([]ComputedAssignment, 0)

	for _, user := range assignedRoles.Users {
		if _, ok := assignmentOrder.Users[user.Name]; ok {
			computedUsers = append(computedUsers, ComputedAssignment{
				Name:  user.Name,
				Roles: user.Roles,
			})
		}
	}

	for _, group := range assignedRoles.Groups {
		if _, ok := assignmentOrder.Groups[group.Name]; ok {
			computedGroups = append(computedGroups, ComputedAssignment{
				Name:  group.Name,
				Roles: group.Roles,
			})
		}
	}

	return createAssignmentResult(ctx, computedUsers, computedGroups)
}

func createAssignmentResult(ctx context.Context, computedUsers []ComputedAssignment, computedGroups []ComputedAssignment) (*AssignmentResult, diag.Diagnostics) {
	computedUsersList, diags := createTfList(ctx, computedUsers)
	if diags != nil {
		return nil, diags
	}

	computedGroupsList, diags := createTfList(ctx, computedGroups)
	if diags != nil {
		return nil, diags
	}

	return &AssignmentResult{
		ComputedUsers:  *computedUsersList,
		ComputedGroups: *computedGroupsList,
	}, nil
}

func createTfList(ctx context.Context, assignments []ComputedAssignment) (*basetypes.ListValue, diag.Diagnostics) {
	slices.SortFunc(assignments, func(a, b ComputedAssignment) int {
		return strings.Compare(a.Name, b.Name)
	})
	for _, assignment := range assignments {
		slices.SortFunc(assignment.Roles, func(a, b string) int {
			return strings.Compare(a, b)
		})
	}

	computedUsersList, diags := types.ListValueFrom(ctx, computedAssignmentType, assignments)
	if diags != nil {
		return nil, diags
	}

	return &computedUsersList, nil
}
