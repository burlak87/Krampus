export interface User {
    id: string,
    name: string,
    secondName: string,
    userName: string,
    phone: string,
    birthday: string,
    country: string,
    bio: string,
    logo: string,
    email: string,
    password: string
}

export interface CallParticipant extends User {
    audio: boolean;
    video: boolean;
    muth: boolean;  
}

export interface Setting {
    chatBackup: "on" | "off",
    theme: "light" | "dark"
}



export interface genericRef<typeData> extends Ref {
    value: typeData | undefined
}

