package confluence

import (
	"context"
	"github.com/emirpasic/gods/utils"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/yunarta/golang-quality-of-life-pack/collections"
	"github.com/yunarta/terraform-atlassian-api-client/confluence"
	"github.com/yunarta/terraform-atlassian-api-client/jira/cloud"
	"slices"
	"strings"
)

type Assignment struct {
	Users       []string `tfsdk:"users"`
	Groups      []string `tfsdk:"groups"`
	Permissions []string `tfsdk:"permissions"`
	Priority    int64    `tfsdk:"priority"`
}

type AssignmentOrder struct {
	Permissions []string
	Users       map[string][]string
	UserNames   []string
	Groups      map[string][]string
	GroupNames  []string
}

type Assignments []Assignment

type UpdateUserPermissionsFunc func(user string, requestedPermissions []string) error
type UpdateGroupPermissionsFunc func(group string, requestedPermissions []string) error

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
	var permissions = make([]string, 0)
	for _, priority := range priorities {
		assignment := makeAssignments[priority]
		for _, user := range assignment.Users {
			usersAssignments[user] = assignment.Permissions
			userNames = append(userNames, user)
			permissions = append(permissions, assignment.Permissions...)
		}

		for _, group := range assignment.Groups {
			groupsAssignments[group] = assignment.Permissions
			groupNames = append(groupNames, group)
			permissions = append(permissions, assignment.Permissions...)
		}
	}

	return &AssignmentOrder{
		Permissions: collections.Unique(permissions),
		Users:       usersAssignments,
		UserNames:   userNames,
		Groups:      groupsAssignments,
		GroupNames:  groupNames,
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
				"permissions": schema.ListAttribute{
					Required:    true,
					ElementType: types.StringType,
					Validators: []validator.List{
						listvalidator.ValueStringsAre(stringvalidator.OneOf(
							"read_space",
							"delete_space",
							"create_page",
							"delete_page",
							"archive_page",
							"create_blogpost",
							"delete_blogpost",
							"delete_comment",
							"create_comment",
							"create_attachment",
							"delete_attachment",
							"administer_space",
							"restrict_content_space",
							"export_space",
							"administer_space",
						)),
					},
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
			"permissions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	},
}

type ComputedAssignment struct {
	Name        string   `tfsdk:"name"`
	Permissions []string `tfsdk:"permissions"`
}

var assignmentType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"permissions": types.ListType{
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
		"permissions": types.ListType{ElemType: types.StringType},
		"name":        types.StringType,
	},
}

type AssignmentResult struct {
	ComputedUsers  types.List
	ComputedGroups types.List
}

func ApplyNewAssignmentSet(ctx context.Context, actorLookupService *cloud.ActorLookupService,
	assignmentOrder AssignmentOrder,
	updateUserPermissions UpdateUserPermissionsFunc,
	updateGroupPermissions UpdateGroupPermissionsFunc) (*AssignmentResult, diag.Diagnostics) {

	var err error

	computedUsers := make([]ComputedAssignment, 0)
	computedGroups := make([]ComputedAssignment, 0)

	for user, requestedPermissions := range assignmentOrder.Users {
		found := actorLookupService.FindUser(user)
		if found == nil {
			continue
		}

		computedUsers = append(computedUsers, ComputedAssignment{
			Name:        user,
			Permissions: requestedPermissions,
		})

		err = updateUserPermissions(user, requestedPermissions)
		if err != nil {
			return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToUpdateUserPermissions, err.Error())}
		}
	}

	for group, requestedPermissions := range assignmentOrder.Groups {
		found := actorLookupService.FindGroup(group)
		if found == nil {
			continue
		}

		computedGroups = append(computedGroups, ComputedAssignment{
			Name:        group,
			Permissions: requestedPermissions,
		})

		err = updateGroupPermissions(group, requestedPermissions)
		if err != nil {
			return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToUpdateGroupPermissions, err.Error())}
		}
	}

	return createAssignmentResult(ctx, computedUsers, computedGroups)
}

func UpdateAssignment(ctx context.Context, actorLookupService *cloud.ActorLookupService,
	inStateAssignmentOrder AssignmentOrder,
	plannedAssignmentOrder AssignmentOrder,
	forceUpdate bool,
	updateUserPermission UpdateUserPermissionsFunc,
	updateGroupPermission UpdateGroupPermissionsFunc) (*AssignmentResult, diag.Diagnostics) {

	computedUsers, diags := updateUsers(inStateAssignmentOrder, plannedAssignmentOrder, actorLookupService, forceUpdate, updateUserPermission)
	if diags != nil {
		return nil, diags
	}

	computedGroups, diags := updateGroups(inStateAssignmentOrder, plannedAssignmentOrder, actorLookupService, forceUpdate, updateGroupPermission)
	if diags != nil {
		return nil, diags
	}

	return createAssignmentResult(ctx, computedUsers, computedGroups)
}

func updateUsers(inStateAssignmentOrder AssignmentOrder, plannedAssignmentOrder AssignmentOrder,
	actorLookupService *cloud.ActorLookupService, forceUpdate bool, updateUserPermissions UpdateUserPermissionsFunc) ([]ComputedAssignment, diag.Diagnostics) {
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

		requestedPermissions := plannedAssignmentOrder.Users[user]
		inStatePermissions := inStateAssignmentOrder.Users[user]
		computedUsers = append(computedUsers, ComputedAssignment{
			Name:        user,
			Permissions: requestedPermissions,
		})

		if !collections.EqualsIgnoreOrder(inStatePermissions, requestedPermissions) || forceUpdate {
			err = updateUserPermissions(user, requestedPermissions)
			if err != nil {
				return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToUpdateUserPermissions, err.Error())}
			}
		}
	}

	for _, user := range removing {
		err := updateUserPermissions(user, make([]string, 0))
		if err != nil {
			return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToRemoveUserPermissions, err.Error())}
		}
	}
	return computedUsers, nil
}

func updateGroups(inStateAssignmentOrder AssignmentOrder, plannedAssignmentOrder AssignmentOrder,
	actorLookupService *cloud.ActorLookupService, forceUpdate bool, updateGroupPermissions UpdateGroupPermissionsFunc) ([]ComputedAssignment, diag.Diagnostics) {
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

		requestedPermissions := plannedAssignmentOrder.Groups[group]
		inStatePermissions := inStateAssignmentOrder.Groups[group]
		computedGroups = append(computedGroups, ComputedAssignment{
			Name:        group,
			Permissions: requestedPermissions,
		})

		if !collections.EqualsIgnoreOrder(inStatePermissions, requestedPermissions) || forceUpdate {
			err = updateGroupPermissions(group, requestedPermissions)
			if err != nil {
				return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToUpdateGroupPermissions, err.Error())}
			}
		}
	}

	for _, group := range removing {
		err := updateGroupPermissions(group, make([]string, 0))
		if err != nil {
			return nil, []diag.Diagnostic{diag.NewErrorDiagnostic(failedToRemoveGroupPermissions, err.Error())}
		}
	}

	return computedGroups, nil
}

func RemoveAssignment(ctx context.Context,
	assignedPermissions *confluence.ObjectPermissions, assignmentOrder *AssignmentOrder,
	updateUserPermissions UpdateUserPermissionsFunc,
	updateGroupPermissions UpdateGroupPermissionsFunc) diag.Diagnostics {

	for _, user := range assignedPermissions.Users {
		if _, ok := assignmentOrder.Users[user.Name]; ok {
			err := updateUserPermissions(user.Name, make([]string, 0))
			if err != nil {
				return []diag.Diagnostic{diag.NewErrorDiagnostic(failedToRemoveUserPermissions, err.Error())}
			}
		}
	}

	for _, group := range assignedPermissions.Groups {
		if _, ok := assignmentOrder.Groups[group.Name]; ok {
			err := updateGroupPermissions(group.Name, make([]string, 0))
			if err != nil {
				return []diag.Diagnostic{diag.NewErrorDiagnostic(failedToRemoveGroupPermissions, err.Error())}
			}
		}
	}

	return nil
}

func ComputePermissionAssignments(ctx context.Context,
	assignedPermissions *confluence.ObjectPermissions, assignmentOrder AssignmentOrder) (*AssignmentResult, diag.Diagnostics) {

	computedUsers := make([]ComputedAssignment, 0)
	computedGroups := make([]ComputedAssignment, 0)

	for _, user := range assignedPermissions.Users {
		if _, ok := assignmentOrder.Users[user.Name]; ok {
			computedUsers = append(computedUsers, ComputedAssignment{
				Name:        user.Name,
				Permissions: user.Permissions,
			})
		}
	}

	for _, group := range assignedPermissions.Groups {
		if _, ok := assignmentOrder.Groups[group.Name]; ok {
			computedGroups = append(computedGroups, ComputedAssignment{
				Name:        group.Name,
				Permissions: group.Permissions,
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
	computedUsersList, diags := types.ListValueFrom(ctx, computedAssignmentType, assignments)
	if diags != nil {
		return nil, diags
	}

	return &computedUsersList, nil
}
