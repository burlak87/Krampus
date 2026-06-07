import type {User} from "./other.ts"
export function isUser(data: User | undefined): data is User {
    return data != undefined
}