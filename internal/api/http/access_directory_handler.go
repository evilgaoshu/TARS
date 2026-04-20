package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"tars/internal/api/dto"
)

func groupsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "groups.read", "groups.write"))
		if !ok {
			return
		}
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			items := deps.Access.ListGroupsFiltered(orgFilterFromRequest(r))
			query := parseListQuery(r)
			filtered := filterGroups(items, query.Query)
			pageItems, meta := paginateItems(filtered, query)
			resp := dto.GroupListResponse{Items: make([]dto.Group, 0, len(pageItems)), ListPage: meta}
			for _, item := range pageItems {
				resp.Items = append(resp.Items, groupToDTO(item))
			}
			auditOpsRead(r.Context(), deps, "groups", "", "list", map[string]any{"actor": principal.User.UserID, "count": len(resp.Items)})
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			var req struct {
				Group dto.Group `json:"group"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			group, err := deps.Access.UpsertGroup(groupFromDTO(req.Group))
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "group", group.GroupID, "group_created", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusCreated, groupToDTO(group))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func groupDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "groups.read", "groups.write"))
		if !ok {
			return
		}
		groupID, action := nestedResourcePath(r.URL.Path, "/api/v1/groups/")
		if groupID == "" {
			writeError(w, http.StatusNotFound, "not_found", "group not found")
			return
		}
		switch {
		case r.Method == http.MethodGet && action == "":
			group, found := deps.Access.GetGroup(groupID)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "group not found")
				return
			}
			auditOpsRead(r.Context(), deps, "group", groupID, "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, groupToDTO(group))
		case r.Method == http.MethodPut && action == "":
			var req struct {
				Group dto.Group `json:"group"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			group := groupFromDTO(req.Group)
			group.GroupID = groupID
			updated, err := deps.Access.UpsertGroup(group)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "group", groupID, "group_updated", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, groupToDTO(updated))
		case r.Method == http.MethodPost && (action == "enable" || action == "disable"):
			var req operatorReasonRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " " + action + "d group"
			}
			status := "disabled"
			if action == "enable" {
				status = "active"
			}
			updated, err := deps.Access.SetGroupStatus(groupID, status)
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "group", groupID, "group_"+action+"d", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, groupToDTO(updated))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func usersHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "users.read", "users.write"))
		if !ok {
			return
		}
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			items := deps.Access.ListUsersFiltered(orgFilterFromRequest(r))
			query := parseListQuery(r)
			filtered := filterUsers(items, query.Query)
			pageItems, meta := paginateItems(filtered, query)
			resp := dto.UserListResponse{Items: make([]dto.User, 0, len(pageItems)), ListPage: meta}
			for _, item := range pageItems {
				resp.Items = append(resp.Items, userToDTO(item))
			}
			auditOpsRead(r.Context(), deps, "users", "", "list", map[string]any{"actor": principal.User.UserID, "count": len(resp.Items)})
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			var req struct {
				User dto.User `json:"user"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			user, err := deps.Access.UpsertUser(userFromDTO(req.User))
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "user", user.UserID, "user_created", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusCreated, userToDTO(user))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func userDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "users.read", "users.write"))
		if !ok {
			return
		}
		id, action := nestedResourcePath(r.URL.Path, "/api/v1/users/")
		if id == "" {
			writeError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		switch {
		case r.Method == http.MethodGet && action == "":
			user, found := deps.Access.GetUser(id)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "user not found")
				return
			}
			auditOpsRead(r.Context(), deps, "user", id, "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, userToDTO(user))
		case r.Method == http.MethodPut && action == "":
			var req struct {
				User dto.User `json:"user"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			user := userFromDTO(req.User)
			user.UserID = id
			updated, err := deps.Access.UpsertUser(user)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "user", id, "user_updated", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, userToDTO(updated))
		case r.Method == http.MethodPost && (action == "enable" || action == "disable"):
			var req operatorReasonRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " " + action + "d user"
			}
			status := "disabled"
			if action == "enable" {
				status = "active"
			}
			updated, err := deps.Access.SetUserStatus(id, status)
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "user", id, "user_"+action+"d", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, userToDTO(updated))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func rolesHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "roles.read", "roles.write"))
		if !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
			items := deps.Access.ListRoles()
			resp := dto.RoleListResponse{Items: make([]dto.Role, 0, len(items))}
			for _, item := range items {
				resp.Items = append(resp.Items, roleToDTO(item))
			}
			auditOpsRead(r.Context(), deps, "roles", "", "list", map[string]any{"actor": principal.User.UserID, "count": len(resp.Items)})
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			var req struct {
				Role dto.Role `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			role, err := deps.Access.UpsertRole(roleFromDTO(req.Role))
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "role", role.ID, "role_created", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusCreated, roleToDTO(role))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func roleDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "roles.read", "roles.write"))
		if !ok {
			return
		}
		roleID, action := nestedResourcePath(r.URL.Path, "/api/v1/roles/")
		if roleID == "" {
			writeError(w, http.StatusNotFound, "not_found", "role not found")
			return
		}
		switch {
		case r.Method == http.MethodGet && action == "bindings":
			bindings, found := deps.Access.GetRoleBindings(roleID)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "role not found")
				return
			}
			userIDs := bindings.UserIDs
			if userIDs == nil {
				userIDs = []string{}
			}
			groupIDs := bindings.GroupIDs
			if groupIDs == nil {
				groupIDs = []string{}
			}
			auditOpsRead(r.Context(), deps, "role", roleID, "get_bindings", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, dto.RoleBindingsResponse{RoleID: roleID, UserIDs: userIDs, GroupIDs: groupIDs})
		case r.Method == http.MethodGet && action == "":
			role, found := deps.Access.GetRole(roleID)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "role not found")
				return
			}
			auditOpsRead(r.Context(), deps, "role", roleID, "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, roleToDTO(role))
		case r.Method == http.MethodPut && action == "":
			var req struct {
				Role dto.Role `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			role := roleFromDTO(req.Role)
			role.ID = roleID
			updated, err := deps.Access.UpsertRole(role)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "role", roleID, "role_updated", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, roleToDTO(updated))
		case r.Method == http.MethodPost && action == "bindings":
			var req dto.RoleBindingRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " updated role bindings"
			}
			if err := deps.Access.SetRoleBindings(roleID, req.UserIDs, req.GroupIDs); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "role", roleID, "role_bindings_updated", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			role, _ := deps.Access.GetRole(roleID)
			writeJSON(w, http.StatusOK, roleToDTO(role))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func peopleHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "people.read", "people.write"))
		if !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := parseListQuery(r)
			items := filterPeople(deps.Access.ListPeopleFiltered(orgFilterFromRequest(r)), query.Query)
			pageItems, meta := paginateItems(items, query)
			resp := dto.PersonListResponse{Items: make([]dto.Person, 0, len(pageItems)), ListPage: meta}
			for _, item := range pageItems {
				resp.Items = append(resp.Items, personToDTO(item))
			}
			auditOpsRead(r.Context(), deps, "people", "", "list", map[string]any{"actor": principal.User.UserID, "count": len(resp.Items)})
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			var req struct {
				Person dto.Person `json:"person"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			item, err := deps.Access.UpsertPerson(personFromDTO(req.Person))
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "person", item.ID, "person_created", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusCreated, personToDTO(item))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func personDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "people.read", "people.write"))
		if !ok {
			return
		}
		id, action := nestedResourcePath(r.URL.Path, "/api/v1/people/")
		if id == "" {
			writeError(w, http.StatusNotFound, "not_found", "person not found")
			return
		}
		switch {
		case r.Method == http.MethodGet && action == "":
			item, found := deps.Access.GetPerson(id)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "person not found")
				return
			}
			auditOpsRead(r.Context(), deps, "person", id, "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, personToDTO(item))
		case r.Method == http.MethodPut && action == "":
			var req struct {
				Person dto.Person `json:"person"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			item := personFromDTO(req.Person)
			item.ID = id
			updated, err := deps.Access.UpsertPerson(item)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "person", id, "person_updated", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, personToDTO(updated))
		case r.Method == http.MethodPost && (action == "enable" || action == "disable"):
			var req operatorReasonRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " " + action + "d person"
			}
			status := "disabled"
			if action == "enable" {
				status = "active"
			}
			updated, err := deps.Access.SetPersonStatus(id, status)
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "person", id, "person_"+action+"d", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, personToDTO(updated))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func channelsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "channels.read", "channels.write"))
		if !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := parseListQuery(r)
			items := filterChannels(deps.Access.ListChannelsFiltered(orgFilterFromRequest(r)), query.Query)
			pageItems, meta := paginateItems(items, query)
			resp := dto.ChannelListResponse{Items: make([]dto.Channel, 0, len(pageItems)), ListPage: meta}
			for _, item := range pageItems {
				resp.Items = append(resp.Items, channelToDTO(item))
			}
			auditOpsRead(r.Context(), deps, "channels", "", "list", map[string]any{"actor": principal.User.UserID, "count": len(resp.Items)})
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			var req struct {
				Channel dto.Channel `json:"channel"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			item, err := deps.Access.UpsertChannel(channelFromDTO(req.Channel))
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "channel", item.ID, "channel_created", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusCreated, channelToDTO(item))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func channelDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "channels.read", "channels.write"))
		if !ok {
			return
		}
		id, action := nestedResourcePath(r.URL.Path, "/api/v1/channels/")
		if id == "" {
			writeError(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		switch {
		case r.Method == http.MethodGet && action == "":
			item, found := deps.Access.GetChannel(id)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "channel not found")
				return
			}
			auditOpsRead(r.Context(), deps, "channel", id, "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, channelToDTO(item))
		case r.Method == http.MethodPut && action == "":
			var req struct {
				Channel dto.Channel `json:"channel"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			item := channelFromDTO(req.Channel)
			item.ID = id
			updated, err := deps.Access.UpsertChannel(item)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "channel", id, "channel_updated", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, channelToDTO(updated))
		case r.Method == http.MethodPost && (action == "enable" || action == "disable"):
			var req operatorReasonRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " " + action + "d channel"
			}
			updated, err := deps.Access.SetChannelEnabled(id, action == "enable")
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "channel", id, "channel_"+action+"d", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, channelToDTO(updated))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}
