export interface AuthRespons {
    sucess: boolean,
    data?: object | string,
    error?: string
}

export type AppErrorCode =
    | "INVALID_MESSAGE"
    | "UNAUTHORIZED"
    | "FORBIDDEN"
    | "ROOM_NOT_FOUND"
    | "USER_NOT_FOUND"
    | "DUPLICATE_ERROR"
    | "VALIDATION_ERROR"
    | "RATE_LIMIT"
    | "PAYLOAD_TOO_LARGE"
    | "TIMEOUT_ERROR"
    | "STORAGE_ERROR"
    | "CONNECTION_ERROR"
    | "INTERNAL_ERROR"

export interface AppError {
    code: AppErrorCode
    message: string
}

export interface TokenResponse {
    access_token: string
    refresh_token: string
}

export interface TwoFARequired {
    temp_token: string
    requires_two_fa: boolean
}

export type SigninResponse = TokenResponse | TwoFARequired

export interface User {
    id: number
    username: string
    firstname: string
    lastname: string
    email: string
    avatar?: string
    two_fa_enabled: boolean
    created_at: string
    status: string
}

export type RoomType = "personal" | "private" | "group" | "video_call"

export interface Room {
    id: string
    type: RoomType
    owner_id: string
    name: string
    members: string[]
    settings: object
    avatar?: string
    created_at: number
    updated_at: number
}

export interface BaseMessage {
    id: string
    type: string
    user_id: string
    room_id: string
    timestamp: number
    version: number
    payload: object
    reply_to_id?: string | null
    forwarded_from_id?: string | null
    topic_id?: string | null
    metadata: object
}

export interface SendMessageResponse {
    status: string
    msg_id: string
}

export interface JoinRoomResponse {
    status: string
    room: Room
}

export interface SearchResult {
    message_id: string
    room_id: string
    user_id: string
    content: string
    created_at: string
}

export interface PollOptionResult {
    id: string
    votes: number
}

export interface PollResults {
    poll_id: string
    options: PollOptionResult[]
}

export interface Sticker {
    id: string
    emoji: string
}

export interface StickerPack {
    id: string
    title: string
    stickers: Sticker[]
}

export interface SyncResponse {
    last_event_sequence: number
}

export interface Profile {
    user_id: string
    avatar?: string
}

export interface ICEServer {
    urls: string[]
    username?: string
    credential?: string
}

export interface ICEServersResponse {
    iceServers: ICEServer[]
}

export interface StatusResponse {
    status: string
}

export interface SuccessResponse {
    success: boolean
}

export interface HealthResponse {
    status: string
    timestamp: string
}
