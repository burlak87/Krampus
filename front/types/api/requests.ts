// Request bodies — mirror components.schemas / requestBody of docs/openapi.yaml.

export interface SignupRequest {
    username: string
    firstname?: string
    lastname?: string
    email: string
    password: string
    two_fa_enabled?: boolean
}

export interface SigninRequest {
    email: string
    password: string
}

export interface VerifyRequest {
    temp_token: string
    code: string
}

export interface RefreshRequest {
    refresh_token: string
}

export interface LogoutRequest {
    password: string
}

export interface SendMessageRequest {
    room_id: string
    type: string
    payload: object
}

export type RoomType = "personal" | "private" | "group" | "video_call"

export interface CreateRoomRequest {
    id: string
    type: RoomType
    name: string
    members?: string[]
}

export interface JoinRoomRequest {
    token: string
}

export interface VoteRequest {
    option_id: string
}

export interface ReactionRequest {
    emoji: string
}

export interface BanRequest {
    user_id: string
    reason: string
}

export interface MuteRequest {
    user_id: string
    duration_seconds?: number
}
