

import type {
    BanRequest,
    CreateRoomRequest,
    JoinRoomRequest,
    MuteRequest,
    ReactionRequest,
    RefreshRequest,
    SendMessageRequest,
    SigninRequest,
    SignupRequest,
    VerifyRequest,
    VoteRequest,
} from "~~/types/api/requests"
import type {
    AppError,
    BaseMessage,
    HealthResponse,
    JoinRoomResponse,
    PollResults,
    Profile,
    Room,
    SearchResult,
    SendMessageResponse,
    SigninResponse,
    StatusResponse,
    StickerPack,
    SuccessResponse,
    SyncResponse,
    TokenResponse,
    User,
} from "~~/types/api/respons"

const API_BASE = "http://localhost:8080"

const ACCESS_TOKEN_KEY = "access_token"
const REFRESH_TOKEN_KEY = "refresh_token"

export class ApiError extends Error {
    code: string
    status: number
    constructor(status: number, body: AppError | undefined) {
        super(body?.message ?? `request failed with status ${status}`)
        this.name = "ApiError"
        this.status = status
        this.code = body?.code ?? "INTERNAL_ERROR"
    }
}

function getToken(): string | null {
    if (typeof localStorage === "undefined") return null
    return localStorage.getItem(ACCESS_TOKEN_KEY)
}

export function setTokens(tokens: TokenResponse) {
    if (typeof localStorage === "undefined") return
    localStorage.setItem(ACCESS_TOKEN_KEY, tokens.access_token)
    localStorage.setItem(REFRESH_TOKEN_KEY, tokens.refresh_token)
}

export function clearTokens() {
    if (typeof localStorage === "undefined") return
    localStorage.removeItem(ACCESS_TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
}

interface RequestOptions {
    method?: string
    body?: unknown
    auth?: boolean
    query?: Record<string, string | number | undefined>
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
    const { method = "GET", body, auth = true, query } = opts

    let url = API_BASE + path
    if (query) {
        const params = new URLSearchParams()
        for (const [k, v] of Object.entries(query)) {
            if (v !== undefined) params.set(k, String(v))
        }
        const qs = params.toString()
        if (qs) url += `?${qs}`
    }

    const headers: Record<string, string> = {}
    const init: RequestInit = { method, headers }

    if (body instanceof FormData) {
        init.body = body
    } else if (body !== undefined) {
        headers["Content-Type"] = "application/json"
        init.body = JSON.stringify(body)
    }

    if (auth) {
        const token = getToken()
        if (token) headers["Authorization"] = `Bearer ${token}`
    }

    const res = await fetch(url, init)

    if (!res.ok) {
        let errBody: AppError | undefined
        try {
            errBody = await res.json()
        } catch {
            errBody = undefined
        }
        throw new ApiError(res.status, errBody)
    }

    if (res.status === 204) return undefined as T
    const text = await res.text()
    return (text ? JSON.parse(text) : undefined) as T
}

export const apiClient = {

    health: () => request<HealthResponse>("/health", { auth: false }),


    signup: (req: SignupRequest) =>
        request<User>("/api/v1/auth/signup", { method: "POST", body: req, auth: false }),

    signin: (req: SigninRequest) =>
        request<SigninResponse>("/api/v1/auth/signin", { method: "POST", body: req, auth: false }),

    verify: (req: VerifyRequest) =>
        request<TokenResponse>("/api/v1/auth/verify", { method: "POST", body: req, auth: false }),

    refresh: (req: RefreshRequest) =>
        request<TokenResponse>("/api/v1/auth/refresh", { method: "POST", body: req, auth: false }),

    logout: (password: string) =>
        request<SuccessResponse>("/api/v1/auth/logout", { method: "POST", body: { password } }),

    enableTwoFA: () =>
        request<SuccessResponse>("/api/v1/auth/enable", { method: "POST" }),


    sendMessage: (req: SendMessageRequest) =>
        request<SendMessageResponse>("/api/v1/chat/messages", { method: "POST", body: req }),

    getRoomMessages: (roomId: string, limit?: number) =>
        request<BaseMessage[]>(`/api/v1/chat/rooms/${roomId}/messages`, { query: { limit } }),

    searchMessages: (roomId: string, q: string, limit?: number) =>
        request<SearchResult[]>(`/api/v1/chat/rooms/${roomId}/search`, { query: { q, limit } }),


    createRoom: (req: CreateRoomRequest) =>
        request<Room>("/api/v1/chat/rooms", { method: "POST", body: req }),

    listRooms: () => request<Room[]>("/api/v1/chat/rooms"),

    getRoom: (roomId: string) => request<Room>(`/api/v1/chat/rooms/${roomId}`),

    joinRoom: (req: JoinRoomRequest) =>
        request<JoinRoomResponse>("/api/v1/chat/rooms/join", { method: "POST", body: req }),


    listUsers: (limit?: number, offset?: number) =>
        request<User[]>("/api/v1/chat/users", { query: { limit, offset } }),

    getUser: (userId: string) => request<User>(`/api/v1/chat/users/${userId}`),


    votePoll: (roomId: string, pollId: string, req: VoteRequest) =>
        request<StatusResponse>(`/api/v1/chat/rooms/${roomId}/polls/${pollId}/vote`, {
            method: "POST",
            body: req,
        }),

    getPoll: (roomId: string, pollId: string) =>
        request<PollResults>(`/api/v1/chat/rooms/${roomId}/polls/${pollId}`),


    addReaction: (messageId: string, req: ReactionRequest) =>
        request<StatusResponse>(`/api/v1/chat/messages/${messageId}/reactions`, {
            method: "POST",
            body: req,
        }),


    listStickers: () => request<StickerPack[]>("/api/v1/chat/stickers"),


    sync: (deviceId: string) =>
        request<SyncResponse>("/api/v1/chat/sync", { query: { device_id: deviceId } }),


    ban: (req: BanRequest) =>
        request<StatusResponse>("/api/v1/chat/moderation/ban", { method: "POST", body: req }),

    mute: (req: MuteRequest) =>
        request<StatusResponse>("/api/v1/chat/moderation/mute", { method: "POST", body: req }),


    uploadAvatar: (avatar: string) =>
        request<Profile>("/api/v1/chat/profile/avatar", { method: "POST", body: { avatar } }),


    setRoomAvatar: (roomId: string, avatar: string) =>
        request<Room>(`/api/v1/chat/rooms/${roomId}/avatar`, { method: "POST", body: { avatar } }),

    getProfile: (userId: string) =>
        request<Profile>(`/api/v1/chat/profile/${userId}`),


}

export type ApiClient = typeof apiClient
