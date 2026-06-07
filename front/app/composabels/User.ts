import { apiClient } from "./apiClient"
import type { User as ApiUser } from "~~/types/api/respons"

export class User {

    constructor() {

    }

    // GET /api/v1/chat/users/{user_id}
    async getUserById(idUser: string): Promise<ApiUser | null> {
        try {
            return await apiClient.getUser(idUser)
        } catch {
            return null
        }
    }

    // GET /api/v1/chat/users
    async getAllUser(limit?: number, offset?: number): Promise<ApiUser[]> {
        try {
            return await apiClient.listUsers(limit, offset)
        } catch {
            return []
        }
    }
}
