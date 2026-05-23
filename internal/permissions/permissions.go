package permissions

type Permission string

const (
	PermissionDeleteMessage Permission = "delete_message"
	PermissionBanUsers      Permission = "ban_users"
	PermissionPinMessages   Permission = "pin_messages"
	PermissionEditMessages  Permission = "edit_messages"
	PermissionInviteUsers   Permission = "invite_users"
	PermissionCreateTopics  Permission = "create_topics"
)
