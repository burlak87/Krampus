import { apiClient } from "./apiClient"
import type { BaseMessage, SearchResult } from "~~/types/api/respons"

export class messageManagment {
    constructor() {

    }

    async createNewMessage(messageData: string, idRoom: string) {
        return apiClient.sendMessage({
            room_id: idRoom,
            type: "text",
            payload: { text: messageData },
        })
    }

    async deleteMessage(idMessage: string, idRoom: string) {
        void idMessage
        void idRoom
    }

    async editMessage(idMessage: string, editData: string, idRoom: string) {
        void idMessage
        void editData
        void idRoom
    }

    async requestAllMessageGroupChat(idRoom: string, limit?: number): Promise<BaseMessage[]> {
        try {
            return await apiClient.getRoomMessages(idRoom, limit)
        } catch {
            return []
        }
    }

    async getUserChatMessage(idRoom: string, limit?: number): Promise<BaseMessage[]> {
        try {
            return await apiClient.getRoomMessages(idRoom, limit)
        } catch {
            return []
        }
    }

    async searchMessages(idRoom: string, query: string, limit?: number): Promise<SearchResult[]> {
        try {
            return await apiClient.searchMessages(idRoom, query, limit)
        } catch {
            return []
        }
    }
}
