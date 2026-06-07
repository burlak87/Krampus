import { apiClient } from "./apiClient"
import type { Room, User as ApiUser } from "~~/types/api/respons"
import type { RoomType } from "~~/types/api/requests"

export class GroupManagment {
    #activeGroup: string | null = null

    constructor(idRoom?: String) {
        if (idRoom) this.#activeGroup = String(idRoom)
    }

    #validationData(): boolean {
        return true
    }

    // POST /api/v1/chat/rooms — a "group" is a room of type "group".
    async createGroup(nameGroup: String): Promise<Room | null> {
        const payload = {
            id: crypto.randomUUID(),
            type: "group" as const,
            name: String(nameGroup),
        }
        console.log("[createGroup] sending", payload)
        try {
            const room = await apiClient.createRoom(payload)
            console.log("[createGroup] created", room)
            return room
        } catch (e) {
            console.error("[createGroup] failed", e)
            return null
        }
    }

    // POST /api/v1/chat/rooms — a 1-on-1 personal chat. The creator is added
    // automatically by the backend; we pass the other participant in members.
    async createPersonal(userId: string, name: string): Promise<Room | null> {
        const payload: any = {
            id: crypto.randomUUID(),
            type: "personal" as const,
            name,
        }
        if (userId) payload.members = [String(userId)]
        console.log("[createPersonal] sending", payload)
        try {
            const room = await apiClient.createRoom(payload)
            console.log("[createPersonal] created", room)
            return room
        } catch (e) {
            console.error("[createPersonal] failed", e)
            return null
        }
    }

    // POST /api/v1/chat/rooms — generic room creation (used for sub-chats).
    async createChat(nameChat: string, typeChat: string, _idFolder: string, _idRoom: string, members: string[] = []): Promise<Room | null> {
        const type: RoomType = (["personal", "private", "group", "video_call"] as const)
            .includes(typeChat as RoomType) ? (typeChat as RoomType) : "private"
        const payload: any = { id: crypto.randomUUID(), type, name: nameChat }
        if (members.length) payload.members = members
        try {
            return await apiClient.createRoom(payload)
        } catch {
            return null
        }
    }

    // Folders, roles, and per-room user management have no REST operation in the
    // OpenAPI spec, so they are not part of the integration layer.
    createFolder(_nameFolder: string, _idRoom: string) {}
    createRole(_nameRole: string, _setting: object, _idRoom: string) {}
    settingGroup() {}
    deleteUserInGroup(_idGroup: string, _idUser: string) {}
    requreAllRole(_idGroup: string) {}
    deleteRoleInGroup(_idGroup: string, _idRole: string) {}

    // GET /api/v1/chat/users
    async requreAllUser(): Promise<ApiUser[]> {
        try {
            return await apiClient.listUsers()
        } catch {
            return []
        }
    }

    // GET /api/v1/chat/rooms/{room_id}
    async openGroup(idGroup: string): Promise<Room | null> {
        try {
            return await apiClient.getRoom(idGroup)
        } catch {
            return null
        }
    }

    // GET /api/v1/chat/rooms
    async requestGroup(): Promise<Room[]> {
        try {
            return await apiClient.listRooms()
        } catch {
            return []
        }
    }

    // POST /api/v1/chat/rooms/join — accepts a deep link or a bare room id.
    async addNewUser(idGroup: string, _userEmail: string): Promise<Room | null> {
        try {
            const res = await apiClient.joinRoom({ token: idGroup })
            return res.room
        } catch {
            return null
        }
    }

    // GET /api/v1/chat/rooms/{room_id}/messages
    async requreAllMessageGroup(idGroup: string) {
        try {
            return await apiClient.getRoomMessages(idGroup)
        } catch {
            return []
        }
    }
}
