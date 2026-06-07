export interface StatusMessage {
                        type: "Status"
                        name: String
                        idUserTarget: String
                        statusUser: "Active" | "Expectation" | "Close" | null
                        audio: boolean 
                        video: boolean 
                        muth: boolean
                        idRoom: string
                        Priority: Number
                        viewingStream: {
                            status: boolean
                            idStreamer: String
                        }
                        offer?: object
                        system_option: object
                    }

export interface answerMessage {
                        type: "Answer"
                        idUserAnswer: String
                        idUserTarget: String
                        action: "Expectation" | "Active" | "Close" | "Video" | "Audio" | "StreamScreen" | null
                        stream_option: object
                        status: "Sent" | "Delivered" | "Opened" | "Hard_bounced" | null
                    }
            
export interface startStreamMessage {
                        type: "StartStream",
                        idRoom: String,
                        maxUser: Number,
                        system_option: {
                            isAudio: boolean,
                            isAudioRole: object,
                            isVideo: boolean,
                            isVideoRole: object,
                            adminRoom: String,
                            timeStartStream: String
                        }
                    }

export interface checkUserActive {
    type: "checkUserActive",
    idUserTarget: String,
    idRoom: String,
    preliminary?: boolean
}

export interface iceCandidate {
    type: "iceCandidate",
    idUserTarget: String,
    idUserAnswer: String,
    iceCandidate: object
}