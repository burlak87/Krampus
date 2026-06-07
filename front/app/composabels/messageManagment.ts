import { apiClient } from "./apiClient"
import type { BaseMessage, SearchResult } from "~~/types/api/respons"

export class messageManagment {
    constructor() {

    }

    // POST /api/v1/chat/messages
    async createNewMessage(messageData: string, idRoom: string) {
        return apiClient.sendMessage({
            room_id: idRoom,
            type: "text",
            payload: { text: messageData },
        })
    }

    // The OpenAPI spec exposes no REST delete/edit message operations; these are
    // realtime-only frames over /ws. Left as no-ops on the REST integration layer.
    async deleteMessage(idMessage: string, idRoom: string) {
        void idMessage
        void idRoom
    }

    async editMessage(idMessage: string, editData: string, idRoom: string) {
        void idMessage
        void editData
        void idRoom
    }

    // GET /api/v1/chat/rooms/{room_id}/messages
    async requestAllMessageGroupChat(idRoom: string, limit?: number): Promise<BaseMessage[]> {
        try {
            return await apiClient.getRoomMessages(idRoom, limit)
        } catch {
            return []
        }
    }

    // GET /api/v1/chat/rooms/{room_id}/messages — messages are addressed by room.
    async getUserChatMessage(idRoom: string, limit?: number): Promise<BaseMessage[]> {
        try {
            return await apiClient.getRoomMessages(idRoom, limit)
        } catch {
            return []
        }
    }

    // GET /api/v1/chat/rooms/{room_id}/search
    async searchMessages(idRoom: string, query: string, limit?: number): Promise<SearchResult[]> {
        try {
            return await apiClient.searchMessages(idRoom, query, limit)
        } catch {
            return []
        }
    }
}
